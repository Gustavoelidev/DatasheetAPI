package scraper

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/yosssi/gohtml"
)

const (
	SELETOR_DATASHEET = "a:contains('Datasheet')"
	SELETOR_ABA_SPECS = "a:contains('Specification')"
)

func BuscarDadosDaPagina(urlAlvo string) (string, string, error) {
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

func BaixarEProcessarArquivo(url string) (conteudo []byte, nomeOriginal string, err error) {
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
