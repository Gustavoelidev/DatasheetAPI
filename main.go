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

// --- Constantes e Tipos ---

const (
	SELETOR_DATASHEET  = "a:contains('Datasheet')"
	SELETOR_ABA_SPECS  = "a:contains('Specification')"
	ARQUIVO_JSON_DADOS = "monitor_dados_geral.json"
)

type Alvo struct {
	Nome string `json:"Nome"`
	URL  string `json:"URL"`
}

type Config struct {
	SmtpHost          string `json:"SmtpHost"`
	SmtpPorta         int    `json:"SmtpPorta"`
	SmtpUsuario       string `json:"SmtpUsuario"`
	SmtpSenha         string `json:"SmtpSenha"`
	EmailDestinatario []string `json:"EmailDestinatario"`
	Alvos             []Alvo `json:"Alvos"`
}

type RegistroMonitor struct {
	URLArquivo        string    `json:"url_arquivo"`
	HashArquivo       string    `json:"hash_arquivo"`
	HashHTML          string    `json:"hash_html"`
	UltimaVerificacao time.Time `json:"ultima_verificacao"`
}

type BancoDeDados map[string]RegistroMonitor

// --- Função Principal ---

func main() {
	// Cria os diretórios necessários para armazenar os artefatos
	if err := os.MkdirAll("Archives", 0755); err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível criar o diretório 'Archives': %v", err)
	}
	if err := os.MkdirAll("Specifications", 0755); err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível criar o diretório 'Specifications': %v", err)
	}
	if err := os.MkdirAll("Diff_Reports", 0755); err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível criar o diretório 'Diff_Reports': %v", err)
	}

	// Carrega as configurações do programa
	config, err := carregarConfiguracao("config.json")
	if err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível carregar as configurações: %v", err)
	}

	// Carrega o banco de dados com os hashes da última verificação
	registros, err := carregarRegistrosJSON()
	if err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível carregar o banco de dados JSON: %v", err)
	}

	// Mapa para guardar os resultados de cada alvo para o relatório final
	resultadosFinais := make(map[string]string)

	// Itera sobre cada alvo definido no config.json
	for _, alvo := range config.Alvos {
		log.Printf("\n--- Iniciando verificação para: %s (%s) ---", alvo.Nome, alvo.URL)

		registroAtualizado, relatorio, err := executarVerificacaoParaAlvo(alvo, registros[alvo.URL])
		if err != nil {
			log.Printf("ERRO: A verificação para '%s' falhou: %v", alvo.Nome, err)
			resultadosFinais[alvo.Nome] = fmt.Sprintf("Falha na verificação: %v", err)
			continue
		}

		// Guarda o resultado e atualiza o registro no mapa
		resultadosFinais[alvo.Nome] = relatorio
		registros[alvo.URL] = registroAtualizado
		log.Printf("--- Verificação concluída para: %s ---", alvo.Nome)
	}

	// Salva o estado atual no arquivo JSON
	if err := salvarRegistrosJSON(registros); err != nil {
		log.Fatalf("ERRO CRÍTICO: Falha ao salvar o banco de dados JSON: %v", err)
	}

	log.Println("\n--- Todas as verificações foram concluídas com sucesso ---")

	// Envia o e-mail consolidado com o relatório final de todos os alvos
	log.Println("Enviando e-mail com o relatório final...")
	if err := enviarEmailRelatorioFinal(config, resultadosFinais); err != nil {
		log.Printf("ERRO CRÍTICO: Falha ao enviar o e-mail de relatório final: %v", err)
	} else {
		log.Println("E-mail de relatório final enviado com sucesso.")
	}
}

// --- Função de Verificação Principal ---

// executarVerificacaoParaAlvo realiza a verificação completa para um único alvo.
// Ele não envia e-mails, apenas retorna um relatório em string do que aconteceu.
func executarVerificacaoParaAlvo(alvo Alvo, registroAntigo RegistroMonitor) (RegistroMonitor, string, error) {
	nomeArquivoSeguro := sanitizarNomeArquivo(alvo.Nome)
	arquivoHtmlSpecs := fmt.Sprintf("%s_latest_specs.html", nomeArquivoSeguro)
	caminhoCompletoHtml := filepath.Join("Specifications", arquivoHtmlSpecs)
	var relatorio string

	htmlAntigo, err := os.ReadFile(caminhoCompletoHtml)
	if err != nil && !os.IsNotExist(err) {
		return registroAntigo, "", fmt.Errorf("erro ao ler arquivo HTML antigo '%s': %w", caminhoCompletoHtml, err)
	}

	linkDownload, htmlAtualFormatado, err := buscarDadosDaPagina(alvo.URL)
	if err != nil {
		return registroAntigo, "", fmt.Errorf("erro ao buscar dados da página '%s': %w", alvo.URL, err)
	}
	log.Printf("Link de download encontrado: %s", linkDownload)

	conteudoArquivo, nomeOriginal, err := baixarEProcessarArquivo(linkDownload)
	if err != nil {
		return registroAntigo, "", fmt.Errorf("erro ao baixar ou processar o link '%s': %w", linkDownload, err)
	}

	hashArquivoAtual := calcularHash(conteudoArquivo)
	hashHTMLAtual := calcularHash([]byte(htmlAtualFormatado))
	log.Printf("Hash do arquivo atual: %s | Hash do HTML atual: %s", hashArquivoAtual, hashHTMLAtual)

	var mudancas []string
	isPrimeiraExecucao := registroAntigo.HashArquivo == "" && registroAntigo.HashHTML == ""

	if isPrimeiraExecucao {
		log.Println("Primeira verificação para este alvo. Salvando estado inicial.")
		relatorio = "Estado inicial salvo pela primeira vez."
	} else {
		if registroAntigo.HashArquivo != hashArquivoAtual {
			log.Println("!!! MUDANÇA DETECTADA NO ARQUIVO DATASHEET !!!")
			mudancas = append(mudancas, "O arquivo do datasheet foi alterado.")
		} else {
			log.Println("Hash do arquivo não mudou.")
		}

		if registroAntigo.HashHTML != hashHTMLAtual {
			log.Println("!!! MUDANÇA DETECTADA NAS ESPECIFICAÇÕES !!!")
			mudancas = append(mudancas, "A seção de especificações no site foi alterada.")
			diff := gerarDiffHtml(string(htmlAntigo), htmlAtualFormatado)
			paginaCompletaDiff := criarPaginaHtmlDeDiff(diff, alvo.Nome)
			dataHora := time.Now().Format("2006-01-02_15-04-05")
			nomeArquivoDiff := fmt.Sprintf("%s_%s_diff.html", dataHora, nomeArquivoSeguro)
			caminhoAnexo := filepath.Join("Diff_Reports", nomeArquivoDiff)
			log.Printf("Salvando relatório de diferenças em: %s", caminhoAnexo)
			if err := os.WriteFile(caminhoAnexo, []byte(paginaCompletaDiff), 0644); err != nil {
				log.Printf("AVISO: Falha ao salvar o arquivo de diff '%s': %v", caminhoAnexo, err)
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
			return registroAntigo, "", fmt.Errorf("erro ao salvar novo arquivo HTML '%s': %w", caminhoCompletoHtml, err)
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
			relatorio = "Alterações detectadas: " + strings.Join(mudancas, ", ")
		}
	} else {
		relatorio = "Nenhuma alteração detectada."
	}

	return novoRegistro, relatorio, nil
}

// --- Funções de E-mail ---

// enviarEmailRelatorioFinal constrói e envia um único e-mail de resumo com todos os resultados.
func enviarEmailRelatorioFinal(config Config, resultados map[string]string) error {
	var corpo strings.Builder
	corpo.WriteString(`<!DOCTYPE html><html lang="pt-br"><head><meta charset="UTF-8"><title>Relatório de Monitoramento</title>
<style>
body { font-family: sans-serif; line-height: 1.6; color: #3e5055; }
.container { max-width: 800px; margin: 20px auto; padding: 20px; border: 1px solid #FFFFFF; border-radius: 5px; }
h1 { color: #00A335; }
ul { list-style-type: none; padding-left: 0; }
li { margin-bottom: 10px; padding: 10px; border-left: 4px solid #ccc; }
li.alteracao { border-left-color: #d72736; background-color: #EBEEEE; }
li.sem-alteracao { border-left-color: #00A335; background-color: #EBEEEE; }
li.falha { border-left-color: #EAB42A; background-color: #EBEEEE; }
strong { display: block; font-size: 1.1em; }
</style>
</head><body><div class="container">
<h1>Relatório Final de Monitoramento</h1>
<p>Olá! A verificação de todas as páginas foram concluídas em ` + time.Now().Format("02/01/2006 às 15:04:05") + `. Abaixo está o resumo:</p><ul>`)

	for nomeAlvo, resultado := range resultados {
		classeCSS := "alteracao"
		if strings.Contains(resultado, "Nenhuma alteração") {
			classeCSS = "sem-alteracao"
		} else if strings.Contains(resultado, "Falha na verificação") {
			classeCSS = "falha"
		}
		corpo.WriteString(fmt.Sprintf(`<li class="%s"><strong>%s</strong>%s</li>`, classeCSS, nomeAlvo, resultado))
	}

	corpo.WriteString(`</ul><hr><p style="font-size: 0.8em; color: #777;">Este é um e-mail automático.<br>
    Este projeto foi desenvolvido por Gustavo Henrique Eli.</p></div></body></html>`)

	return enviarEmail(config, "Relatório de Monitoramento de Páginas", corpo.String(), "")
}

// enviarEmail é a função base para enviar e-mails usando gomail.
func enviarEmail(config Config, assunto, corpo string, anexo string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", config.SmtpUsuario)
	m.SetHeader("To", config.EmailDestinatario...)
	m.SetHeader("Subject", assunto)
	m.SetBody("text/html", corpo)

	if anexo != "" {
		m.Attach(anexo)
	}

	d := gomail.NewDialer(config.SmtpHost, config.SmtpPorta, config.SmtpUsuario, config.SmtpSenha)
	return d.DialAndSend(m)
}

// --- Funções de Web Scraping e Arquivos ---

// buscarDadosDaPagina extrai o link do datasheet e o HTML da seção de especificações.
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
		return "", "", fmt.Errorf("href da aba specs inválido ou não encontrado")
	}

	htmlBruto, err := doc.Find(idConteudoSpecs).Html()
	if err != nil || htmlBruto == "" {
		return "", "", fmt.Errorf("conteúdo de specs não encontrado para o seletor '%s'", idConteudoSpecs)
	}
	htmlSpecs := gohtml.Format(htmlBruto)
	return linkDownload, htmlSpecs, nil
}

// baixarEProcessarArquivo faz o download de um arquivo a partir de uma URL.
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
		nomeOriginal = "datasheet_" + filepath.Base(url)
		log.Printf("AVISO: Não foi possível determinar o nome original do arquivo. Usando '%s'.", nomeOriginal)
	}

	conteudo, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, "", err
	}
	return conteudo, nomeOriginal, nil
}

// --- Funções Auxiliares e de Dados ---

// carregarConfiguracao lê o arquivo config.json.
func carregarConfiguracao(caminho string) (Config, error) {
	var config Config
	arquivo, err := os.ReadFile(caminho)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(arquivo, &config)
	return config, err
}

// carregarRegistrosJSON lê o banco de dados de hashes.
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

// salvarRegistrosJSON salva o estado atual no banco de dados de hashes.
func salvarRegistrosJSON(registros BancoDeDados) error {
	dados, err := json.MarshalIndent(registros, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ARQUIVO_JSON_DADOS, dados, 0644)
}

// sanitizarNomeArquivo remove caracteres inválidos de um nome de arquivo.
func sanitizarNomeArquivo(nome string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_", "*", "_", "?", "_", "\"", "'", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(nome)
}

// calcularHash gera um hash MD5 para um slice de bytes.
func calcularHash(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// criarPaginaHtmlDeDiff gera uma página HTML completa para exibir as diferenças.
func criarPaginaHtmlDeDiff(conteudoDiff, nomeAlvo string) string {
	titulo := fmt.Sprintf("Relatório de Alterações para %s", nomeAlvo)
	return fmt.Sprintf(`<!DOCTYPE html><html lang="pt-br"><head><meta charset="UTF-8">
<title>%s</title>
<style>
body { font-family: sans-serif; line-height: 1.6; color: #333; }
.container { max-width: 90%%; margin: 20px auto; padding: 20px; border: 1px solid #ddd; border-radius: 5px; }
h1 { color: #555; }
del { background-color: #fdd; text-decoration: none; padding: 2px 0; }
ins { background-color: #dfd; text-decoration: none; padding: 2px 0; }
</style></head><body><div class="container">
<h1>%s</h1><hr>
<p>Abaixo estão as diferenças detalhadas encontradas na seção de especificações.</p>
<div>%s</div></div></body></html>`, titulo, titulo, conteudoDiff)
}

// gerarDiffHtml compara dois textos e retorna um HTML com as diferenças.
func gerarDiffHtml(textoAntigo, textoNovo string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(textoAntigo, textoNovo, true)
	return dmp.DiffPrettyHtml(diffs)
}