package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

func SanitizarNomeArquivo(nome string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_", "*", "_", "?", "_", "\"", "'", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(nome)
}

func CalcularHash(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

func CriarPaginaHtmlDeDiff(conteudoDiff, nomeAlvo string) string {
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

func GerarDiffHtml(textoAntigo, textoNovo string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(textoAntigo, textoNovo, true)
	return dmp.DiffPrettyHtml(diffs)
}
