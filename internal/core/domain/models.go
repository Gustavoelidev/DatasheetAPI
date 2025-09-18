package domain

import "time"

type Alvo struct {
	Nome string `json:"Nome"`
	URL  string `json:"URL"`
}

type Config struct {
	SmtpHost          string   `json:"SmtpHost"`
	SmtpPorta         int      `json:"SmtpPorta"`
	SmtpUsuario       string   `json:"SmtpUsuario"`
	SmtpSenha         string   `json:"SmtpSenha"`
	EmailDestinatario []string `json:"EmailDestinatario"`
	Alvos             []Alvo   `json:"Alvos"`
}

type RegistroMonitor struct {
	URLArquivo        string    `json:"url_arquivo"`
	HashArquivo       string    `json:"hash_arquivo"`
	HashHTML          string    `json:"hash_html"`
	UltimaVerificacao time.Time `json:"ultima_verificacao"`
}

type BancoDeDados map[string]RegistroMonitor
