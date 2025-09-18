package email

import (
	"datasheetApi/internal/core/domain"
	"fmt"
	"strings"
	"time"

	"gopkg.in/gomail.v2"
)

func EnviarEmailRelatorioFinal(config domain.Config, resultados map[string]string) error {
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

func enviarEmail(config domain.Config, assunto, corpo string, anexo string) error {
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
