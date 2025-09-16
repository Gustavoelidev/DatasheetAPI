package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/yosssi/gohtml"
	"gopkg.in/gomail.v2"
)

const SELETOR_DATASHEET = "a:contains('Datasheet')"
const SELETOR_ABA_SPECS = "a:contains('Specification')"
const ARQUIVO_JSON_DADOS = "monitor_dados_geral.json"

type Alvo struct {
	Nome string `json:"Nome"`
	URL  string `json:"URL"`
}

type Config struct {
	SmtpHost          string `json:"SmtpHost"`
	SmtpPorta         int    `json:"SmtpPorta"`
	SmtpUsuario       string `json:"SmtpUsuario"`
	SmtpSenha         string `json:"SmtpSenha"`
	EmailDestinatario string `json:"EmailDestinatario"`
	Alvos             []Alvo `json:"Alvos"`
}

type RegistroMonitor struct {
	URLArquivo        string    `json:"url_arquivo"`
	HashArquivo       string    `json:"hash_arquivo"`
	HashHTML          string    `json:"hash_html"`
	UltimaVerificacao time.Time `json:"ultima_verificacao"`
}

type BancoDeDados map[string]RegistroMonitor

func main() {
	if err := os.MkdirAll("Archives", 0755); err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível criar o diretório 'Archives': %v", err)
	}
	if err := os.MkdirAll("Specifications", 0755); err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível criar o diretório 'Specifications': %v", err)
	}

	config, err := carregarConfiguracao("config.json")
	if err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível carregar as configurações: %v", err)
	}

	registros, err := carregarRegistrosJSON()
	if err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível carregar o banco de dados JSON: %v", err)
	}

	for _, alvo := range config.Alvos {
		log.Printf("\n--- Iniciando verificação para: %s (%s) ---", alvo.Nome, alvo.URL)

		registroAtualizado, err := executarVerificacaoParaAlvo(config, alvo, registros[alvo.URL])
		if err != nil {
			log.Printf("ERRO: A verificação para '%s' falhou: %v", alvo.Nome, err)
			continue
		}

		registros[alvo.URL] = registroAtualizado
		log.Printf("--- Verificação concluída para: %s ---", alvo.Nome)
	}

	if err := salvarRegistrosJSON(registros); err != nil {
		log.Fatalf("ERRO CRÍTICO: Falha ao salvar o banco de dados JSON: %v", err)
	}

	log.Println("\n--- Todas as verificações foram concluídas com sucesso ---")
}

// A função completa e atualizada
func executarVerificacaoParaAlvo(config Config, alvo Alvo, registroAntigo RegistroMonitor) (RegistroMonitor, error) {
	nomeArquivoSeguro := sanitizarNomeArquivo(alvo.Nome)
	arquivoHtmlSpecs := fmt.Sprintf("%s_latest_specs.html", nomeArquivoSeguro)
	caminhoCompletoHtml := filepath.Join("Specifications", arquivoHtmlSpecs)

	htmlAntigo, err := os.ReadFile(caminhoCompletoHtml)
	if err != nil && !os.IsNotExist(err) {
		return registroAntigo, fmt.Errorf("erro ao ler arquivo HTML antigo '%s': %w", caminhoCompletoHtml, err)
	}

	linkDownload, htmlAtualFormatado, err := buscarDadosDaPagina(alvo.URL)
	if err != nil {
		return registroAntigo, fmt.Errorf("erro ao buscar dados da página '%s': %w", alvo.URL, err)
	}
	log.Printf("Link de download encontrado: %s", linkDownload)

	conteudoArquivo, nomeOriginal, err := baixarEProcessarArquivo(linkDownload)
	if err != nil {
		return registroAntigo, fmt.Errorf("erro ao baixar ou processar o link '%s': %w", linkDownload, err)
	}

	hashArquivoAtual := calcularHash(conteudoArquivo)
	log.Printf("Hash do arquivo atual: %s", hashArquivoAtual)
	hashHTMLAtual := calcularHash([]byte(htmlAtualFormatado))
	log.Printf("Hash do HTML atual: %s", hashHTMLAtual)

	var mudancas []string
	var caminhoAnexo string // Variável para guardar o caminho do anexo

	isPrimeiraExecucao := registroAntigo.HashArquivo == "" && registroAntigo.HashHTML == ""

	if isPrimeiraExecucao {
		log.Println("Primeira verificação para este alvo. Salvando estado inicial.")
	} else {
		if registroAntigo.HashArquivo != hashArquivoAtual {
			log.Println("!!! MUDANÇA DETECTADA NO ARQUIVO DATASHEET !!!")
			mudancas = append(mudancas, fmt.Sprintf("O arquivo do datasheet foi alterado. Novo hash: %s", hashArquivoAtual))
		} else {
			log.Println("Hash do arquivo não mudou.")
		}

		if registroAntigo.HashHTML != hashHTMLAtual {
			log.Println("!!! MUDANÇA DETECTADA NAS ESPECIFICAÇÕES !!!")
			mudancas = append(mudancas, "A seção de especificações foi alterada (veja o anexo para detalhes).")
			
			// Gera o diff e cria o arquivo HTML de relatório
			diff := gerarDiffHtml(string(htmlAntigo), htmlAtualFormatado)
			paginaCompletaDiff := criarPaginaHtmlDeDiff(diff, alvo.Nome)

			dataHora := time.Now().Format("2006-01-02_15-04-05")
			nomeArquivoDiff := fmt.Sprintf("%s_%s_diff.html", dataHora, nomeArquivoSeguro)
			caminhoAnexo = filepath.Join("Diff_Reports", nomeArquivoDiff)

			log.Printf("Salvando relatório de diferenças em: %s", caminhoAnexo)
			if err := os.WriteFile(caminhoAnexo, []byte(paginaCompletaDiff), 0644); err != nil {
				log.Printf("AVISO: Falha ao salvar o arquivo de diff '%s': %v", caminhoAnexo, err)
				caminhoAnexo = "" // Não anexa se falhou ao salvar
			}
		} else {
			log.Println("Hash do HTML não mudou.")
		}
	}

	novoRegistro := RegistroMonitor{
		URLArquivo:        linkDownload,
		HashArquivo:       hashArquivoAtual,
		HashHTML:          hashHTMLAtual,
		UltimaVerificacao: time.Now(),
	}

	if isPrimeiraExecucao || len(mudancas) > 0 {
		if err := os.WriteFile(caminhoCompletoHtml, []byte(htmlAtualFormatado), 0644); err != nil {
			return registroAntigo, fmt.Errorf("erro ao salvar novo arquivo HTML '%s': %w", caminhoCompletoHtml, err)
		}

		if conteudoArquivo != nil {
			dataAtual := time.Now().Format("2006-01-02")
			novoNomeArquivo := fmt.Sprintf("%s_%s", dataAtual, nomeOriginal)
			caminhoDestino := filepath.Join("Archives", novoNomeArquivo)
			log.Printf("Salvando datasheet em: %s", caminhoDestino)
			if err := os.WriteFile(caminhoDestino, conteudoArquivo, 0644); err != nil {
				log.Printf("AVISO: Falha ao salvar o arquivo do datasheet '%s': %v", caminhoDestino, err)
			}
		}

		if len(mudancas) > 0 {
			assuntoEmail := fmt.Sprintf("Alerta: Alterações Detectadas em '%s'", alvo.Nome)
			corpoEmail := construirCorpoEmail(mudancas, alvo.URL)
			// A chamada agora inclui o caminho do anexo (que pode estar vazio)
			if err := enviarEmail(config, assuntoEmail, corpoEmail, caminhoAnexo); err != nil {
				log.Printf("AVISO: Não foi possível enviar o e-mail de notificação para '%s': %v", alvo.Nome, err)
			}
		}
	}

	return novoRegistro, nil
}

func carregarConfiguracao(caminho string) (Config, error) {
	var config Config
	arquivo, err := os.ReadFile(caminho)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(arquivo, &config)
	return config, err
}

func enviarEmail(config Config, assunto, corpo string, anexo string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", config.SmtpUsuario)
	m.SetHeader("To", config.EmailDestinatario)
	m.SetHeader("Subject", assunto)
	m.SetBody("text/html", corpo)

	if anexo != "" {
		m.Attach(anexo)
	}

	d := gomail.NewDialer(config.SmtpHost, config.SmtpPorta, config.SmtpUsuario, config.SmtpSenha)

	log.Println("Enviando e-mail de notificação...")
	return d.DialAndSend(m)
}
func sanitizarNomeArquivo(nome string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_", "*", "_", "?", "_", "\"", "'", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(nome)
}

func construirCorpoEmail(mudancas []string, urlPagina string) string {
	var corpo strings.Builder
	corpo.WriteString(`<!DOCTYPE html><html lang="pt-br"><head><meta charset="UTF-8"><title>Alerta de Alteração</title><style>body { font-family: sans-serif; line-height: 1.6; color: #333; } .container { max-width: 800px; margin: 20px auto; padding: 20px; border: 1px solid #ddd; border-radius: 5px; } h1 { color: #d9534f; } ul { list-style-type: disc; margin-left: 20px; }</style></head><body><div class="container"><h1>Alerta de Monitoramento</h1>`)
	corpo.WriteString(fmt.Sprintf("<p>Olá! Seu monitor detectou as seguintes alterações na página: <a href='%s'>%s</a></p><ul>", urlPagina, urlPagina))
	for _, mudanca := range mudancas {
		corpo.WriteString(fmt.Sprintf("<li>%s</li>", mudanca))
	}
	corpo.WriteString(`</ul><p><b>Por favor, verifique o arquivo HTML anexado para ver as diferenças detalhadas nas especificações (se aplicável).</b></p><hr><p style="font-size: 0.8em; color: #777;">Este é um e-mail automático.</p></div></body></html>`)
	return corpo.String()
}

func carregarRegistrosJSON() (BancoDeDados, error) {
	registros := make(BancoDeDados)
	arquivo, err := os.ReadFile(ARQUIVO_JSON_DADOS)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Arquivo JSON de dados não encontrado. Será criado um novo.")
			return registros, nil
		}
		return nil, err
	}
	err = json.Unmarshal(arquivo, &registros)
	return registros, err
}
func criarPaginaHtmlDeDiff(conteudoDiff, nomeAlvo string) string {
	titulo := fmt.Sprintf("Relatório de Alterações para %s", nomeAlvo)
	var corpo strings.Builder
	corpo.WriteString(`<!DOCTYPE html><html lang="pt-br"><head><meta charset="UTF-8">`)
	corpo.WriteString(fmt.Sprintf("<title>%s</title>", titulo))
	corpo.WriteString(`<style>body { font-family: sans-serif; line-height: 1.6; color: #333; } .container { max-width: 90%; margin: 20px auto; padding: 20px; border: 1px solid #ddd; border-radius: 5px; } h1 { color: #555; } del { background-color: #fdd; text-decoration: none; padding: 2px 0; } ins { background-color: #dfd; text-decoration: none; padding: 2px 0; }</style></head><body><div class="container">`)
	corpo.WriteString(fmt.Sprintf("<h1>%s</h1>", titulo))
	corpo.WriteString(`<hr><p>Abaixo estão as diferenças detalhadas encontradas na seção de especificações.</p><div>`)
	corpo.WriteString(conteudoDiff)
	corpo.WriteString(`</div></div></body></html>`)
	return corpo.String()
}
func salvarRegistrosJSON(registros BancoDeDados) error {
	dados, err := json.MarshalIndent(registros, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ARQUIVO_JSON_DADOS, dados, 0644)
}

func buscarDadosDaPagina(urlAlvo string) (string, string, error) {
	res, err := http.Get(urlAlvo)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return "", "", err
	}
	linkDownload, exists := doc.Find(SELETOR_DATASHEET).First().Attr("href")
	if !exists {
		return "", "", fmt.Errorf("seletor de datasheet não encontrado")
	}
	base, _ := url.Parse(urlAlvo)
	downloadURL, _ := base.Parse(linkDownload)
	linkDownload = downloadURL.String()
	abaSpecs := doc.Find(SELETOR_ABA_SPECS).First()
	if abaSpecs.Length() == 0 {
		return "", "", fmt.Errorf("seletor de aba specs não encontrado")
	}
	idConteudoSpecs, exists := abaSpecs.Attr("href")
	if !exists || !strings.HasPrefix(idConteudoSpecs, "#") {
		return "", "", fmt.Errorf("href da aba specs inválido")
	}
	htmlBruto, err := doc.Find(idConteudoSpecs).Html()
	if err != nil || htmlBruto == "" {
		return "", "", fmt.Errorf("conteúdo de specs não encontrado")
	}
	htmlSpecs := gohtml.Format(htmlBruto)
	return linkDownload, htmlSpecs, nil
}

func calcularHash(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

func baixarEProcessarArquivo(url string) (conteudo []byte, nomeOriginal string, err error) {
	if url == "" {
		return nil, "", nil
	}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	disposition := res.Header.Get("Content-Disposition")
	if disposition != "" {
		_, params, err := mime.ParseMediaType(disposition)
		if err == nil {
			nomeOriginal = params["filename"]
		}
	}
	if nomeOriginal == "" {
		nomeOriginal = "datasheet_sem_nome.pdf"
		log.Printf("AVISO: Não foi possível determinar o nome original do arquivo. Usando '%s'.", nomeOriginal)
	}
	conteudo, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, "", err
	}
	return conteudo, nomeOriginal, nil
}

func gerarDiffHtml(textoAntigo, textoNovo string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(textoAntigo, textoNovo, true)
	return dmp.DiffPrettyHtml(diffs)
}