package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/yosssi/gohtml"
	"gopkg.in/gomail.v2"
)

// --- CONFIGURAÇÕES DO PROGRAMA ---
const URL_ALVO = "LINK"
const SELETOR_DATASHEET = "a:contains('Datasheet')"
const SELETOR_ABA_SPECS = "a:contains('Specification')"
const ARQUIVO_JSON_DADOS = "monitor_dados.json"
const ARQUIVO_HTML_SPECS = "latest_specifications.html"

// --- ESTRUTURAS DE DADOS ---

// Config agora guarda as configurações de e-mail lidas do config.json
type Config struct {
	SmtpHost        string `json:"SmtpHost"`
	SmtpPorta       int    `json:"SmtpPorta"`
	SmtpUsuario     string `json:"SmtpUsuario"`
	SmtpSenha       string `json:"SmtpSenha"`
	EmailDestinatario string `json:"EmailDestinatario"`
}

type RegistroMonitor struct {
	URLArquivo        string    `json:"url_arquivo"`
	HashArquivo       string    `json:"hash_arquivo"`
	HashHTML          string    `json:"hash_html"`
	UltimaVerificacao time.Time `json:"ultima_verificacao"`
}

// --- FUNÇÃO PRINCIPAL ---

func main() {
	config, err := carregarConfiguracao("config.json")
	if err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível carregar as configurações: %v", err)
	}

	if err := executarVerificacao(config); err != nil {
		log.Fatalf("ERRO: A verificação falhou: %v", err)
	}
	log.Println("--- Verificação concluída com sucesso ---")
}

func executarVerificacao(config Config) error {
	log.Println("--- Iniciando verificação ---")

	registroAntigo, err := carregarRegistroJSON()
	if err != nil { return fmt.Errorf("erro ao carregar registro JSON: %w", err) }
	
	htmlAntigo, err := os.ReadFile(ARQUIVO_HTML_SPECS)
	if err != nil && !os.IsNotExist(err) { return fmt.Errorf("erro ao ler arquivo HTML antigo: %w", err) }

	linkDownload, htmlAtualFormatado, err := buscarDadosDaPagina()
	if err != nil { return fmt.Errorf("erro ao buscar dados da página: %w", err) }
	log.Printf("Link de download encontrado: %s", linkDownload)

	hashArquivoAtual, err := calcularHashDoLink(linkDownload)
	if err != nil { return fmt.Errorf("erro ao calcular hash do link '%s': %w", linkDownload, err) }
	log.Printf("Hash do arquivo atual: %s", hashArquivoAtual)
	
	hashHTMLAtual := calcularHash([]byte(htmlAtualFormatado))
	log.Printf("Hash do HTML atual: %s", hashHTMLAtual)

	var mudancas []string
	isPrimeiraExecucao := registroAntigo.HashArquivo == "" && registroAntigo.HashHTML == ""

	if isPrimeiraExecucao {
		log.Println("Primeira verificação. Salvando estado inicial.")
	} else {
		if registroAntigo.HashArquivo != hashArquivoAtual {
			log.Println("!!! MUDANÇA DETECTADA NO ARQUIVO DATASHEET !!!")
			mudancas = append(mudancas, fmt.Sprintf("<h2>O arquivo do datasheet foi alterado</h2><p><b>URL:</b> %s</p><p><b>Hash antigo:</b> %s</p><p><b>Hash novo:</b> %s</p>", linkDownload, registroAntigo.HashArquivo, hashArquivoAtual))
		} else { log.Println("Hash do arquivo não mudou.") }

		if registroAntigo.HashHTML != hashHTMLAtual {
			log.Println("!!! MUDANÇA DETECTADA NAS ESPECIFICAÇÕES !!!")
			diff := gerarDiffHtml(string(htmlAntigo), htmlAtualFormatado)
			mudancas = append(mudancas, fmt.Sprintf("<h2>A seção de especificações foi alterada</h2><div><b>Diferenças:</b><br>%s</div>", diff))
		} else { log.Println("Hash do HTML não mudou.") }
	}

	if isPrimeiraExecucao || len(mudancas) > 0 {
		novoRegistro := RegistroMonitor{
			URLArquivo:  linkDownload,
			HashArquivo: hashArquivoAtual,
			HashHTML:    hashHTMLAtual,
		}
		if err := salvarRegistroJSON(novoRegistro); err != nil { return fmt.Errorf("erro ao salvar novo registro JSON: %w", err) }
		if err := os.WriteFile(ARQUIVO_HTML_SPECS, []byte(htmlAtualFormatado), 0644); err != nil { return fmt.Errorf("erro ao salvar novo arquivo HTML: %w", err) }

		if len(mudancas) > 0 {
			corpoEmail := construirCorpoEmail(mudancas)
			if err := enviarEmail(config, "Alerta: Alterações Detectadas na Página da H3C", corpoEmail); err != nil { 
				log.Printf("AVISO: Não foi possível enviar o e-mail de notificação: %v", err) 
			}
		}
	}

	return nil
}

// --- FUNÇÕES AUXILIARES ---

func carregarConfiguracao(caminho string) (Config, error) {
	var config Config
	arquivo, err := os.ReadFile(caminho)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(arquivo, &config)
	return config, err
}

func enviarEmail(config Config, assunto, corpo string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", config.SmtpUsuario)
	m.SetHeader("To", config.EmailDestinatario)
	m.SetHeader("Subject", assunto)
	m.SetBody("text/html", corpo)
	
	d := gomail.NewDialer(config.SmtpHost, config.SmtpPorta, config.SmtpUsuario, config.SmtpSenha)
	
	log.Println("Enviando e-mail de notificação...")
	return d.DialAndSend(m)
}

func construirCorpoEmail(mudancas []string) string {
	var corpo strings.Builder
	corpo.WriteString(`<!DOCTYPE html><html lang="pt-br"><head><meta charset="UTF-8"><title>Alerta de Alteração</title><style>body { font-family: sans-serif; line-height: 1.6; color: #333; } .container { max-width: 800px; margin: 20px auto; padding: 20px; border: 1px solid #ddd; border-radius: 5px; } h1 { color: #d9534f; } h2 { color: #555; border-bottom: 1px solid #eee; padding-bottom: 5px; } del { background-color: #fdd; text-decoration: none; } ins { background-color: #dfd; text-decoration: none; } p, div { margin-bottom: 10px; }</style></head><body><div class="container"><h1>Alerta de Monitoramento</h1><p>Olá! Seu monitor detectou as seguintes alterações na página monitorada:</p>`)
	for _, mudanca := range mudancas { corpo.WriteString(mudanca) }
	corpo.WriteString(`<hr><p style="font-size: 0.8em; color: #777;">Este é um e-mail automático enviado pelo seu sistema de monitoramento.</p></div></body></html>`)
	return corpo.String()
}

func carregarRegistroJSON() (RegistroMonitor, error) {
	var registro RegistroMonitor
	arquivo, err := os.ReadFile(ARQUIVO_JSON_DADOS); if err != nil { if os.IsNotExist(err) { log.Println("Arquivo JSON de dados não encontrado. Será criado um novo."); return registro, nil } ; return registro, err }
	err = json.Unmarshal(arquivo, &registro); return registro, err
}

func salvarRegistroJSON(registro RegistroMonitor) error {
	registro.UltimaVerificacao = time.Now(); dados, err := json.MarshalIndent(registro, "", "  "); if err != nil { return err }; return os.WriteFile(ARQUIVO_JSON_DADOS, dados, 0644)
}

func buscarDadosDaPagina() (string, string, error) {
	res, err := http.Get(URL_ALVO); if err != nil { return "", "", err }; defer res.Body.Close()
	doc, err := goquery.NewDocumentFromReader(res.Body); if err != nil { return "", "", err }
	linkDownload, exists := doc.Find(SELETOR_DATASHEET).First().Attr("href"); if !exists { return "", "", fmt.Errorf("seletor de datasheet não encontrado") }; base, _ := url.Parse(URL_ALVO); downloadURL, _ := base.Parse(linkDownload); linkDownload = downloadURL.String()
	abaSpecs := doc.Find(SELETOR_ABA_SPECS).First(); if abaSpecs.Length() == 0 { return "", "", fmt.Errorf("seletor de aba specs não encontrado") }
	idConteudoSpecs, exists := abaSpecs.Attr("href"); if !exists || !strings.HasPrefix(idConteudoSpecs, "#") { return "", "", fmt.Errorf("href da aba specs inválido") }
	htmlBruto, err := doc.Find(idConteudoSpecs).Html(); if err != nil || htmlBruto == "" { return "", "", fmt.Errorf("conteúdo de specs não encontrado") }
	htmlSpecs := gohtml.Format(htmlBruto)
	return linkDownload, htmlSpecs, nil
}

func calcularHash(data []byte) string { hash := md5.Sum(data); return hex.EncodeToString(hash[:]) }

func calcularHashDoLink(url string) (string, error) {
	if url == "" { return "", nil }; req, _ := http.NewRequest("GET", url, nil); req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	client := &http.Client{}; res, err := client.Do(req); if err != nil { return "", err }; defer res.Body.Close()
	conteudo, err := io.ReadAll(res.Body); if err != nil { return "", err }; return calcularHash(conteudo), nil
}

func gerarDiffHtml(textoAntigo, textoNovo string) string { dmp := diffmatchpatch.New(); diffs := dmp.DiffMain(textoAntigo, textoNovo, true); return dmp.DiffPrettyHtml(diffs) }