package services

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"datasheetApi/internal/core/domain"
	"datasheetApi/internal/infrastructure/scraper"
	"datasheetApi/internal/pkg/utils"
)

func SetupDirectories() {
	if err := os.MkdirAll("Archives", 0755); err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível criar o diretório 'Archives': %v", err)
	}
	if err := os.MkdirAll("Specifications", 0755); err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível criar o diretório 'Specifications': %v", err)
	}
	if err := os.MkdirAll("Diff_Reports", 0755); err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível criar o diretório 'Diff_Reports': %v", err)
	}
}

func VerificarAlvo(alvo domain.Alvo, registroAntigo domain.RegistroMonitor) (domain.RegistroMonitor, string, error) {
	nomeArquivoSeguro := utils.SanitizarNomeArquivo(alvo.Nome)
	arquivoHtmlSpecs := fmt.Sprintf("%s_latest_specs.html", nomeArquivoSeguro)
	caminhoCompletoHtml := filepath.Join("Specifications", arquivoHtmlSpecs)
	var relatorio string

	htmlAntigo, err := os.ReadFile(caminhoCompletoHtml)
	if err != nil && !os.IsNotExist(err) {
		return registroAntigo, "", fmt.Errorf("erro ao ler arquivo HTML antigo '%s': %w", caminhoCompletoHtml, err)
	}

	linkDownload, htmlAtualFormatado, err := scraper.BuscarDadosDaPagina(alvo.URL)
	if err != nil {
		return registroAntigo, "", fmt.Errorf("erro ao buscar dados da página '%s': %w", alvo.URL, err)
	}
	log.Printf("Link de download encontrado: %s", linkDownload)

	conteudoArquivo, nomeOriginal, err := scraper.BaixarEProcessarArquivo(linkDownload)
	if err != nil {
		return registroAntigo, "", fmt.Errorf("erro ao baixar ou processar o link '%s': %w", linkDownload, err)
	}

	hashArquivoAtual := utils.CalcularHash(conteudoArquivo)
	hashHTMLAtual := utils.CalcularHash([]byte(htmlAtualFormatado))
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
			diff := utils.GerarDiffHtml(string(htmlAntigo), htmlAtualFormatado)
			paginaCompletaDiff := utils.CriarPaginaHtmlDeDiff(diff, alvo.Nome)
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

	novoRegistro := domain.RegistroMonitor{
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
