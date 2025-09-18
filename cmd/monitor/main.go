package main

import (
	"log"

	"datasheetApi/internal/config"
	"datasheetApi/internal/core/services"
	"datasheetApi/internal/infrastructure/email"
	"datasheetApi/internal/infrastructure/persistence"
)

func main() {
	services.SetupDirectories()

	cfg, err := config.CarregarConfiguracao("config.json")
	if err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível carregar as configurações: %v", err)
	}

	registros, err := persistence.CarregarRegistrosJSON()
	if err != nil {
		log.Fatalf("ERRO CRÍTICO: Não foi possível carregar o banco de dados JSON: %v", err)
	}

	resultadosFinais := make(map[string]string)

	for _, alvo := range cfg.Alvos {
		log.Printf("\n--- Iniciando verificação para: %s (%s) ---", alvo.Nome, alvo.URL)

		registroAtualizado, relatorio, err := services.VerificarAlvo(alvo, registros[alvo.URL])
		if err != nil {
			log.Printf("ERRO: A verificação para '%s' falhou: %v", alvo.Nome, err)
			resultadosFinais[alvo.Nome] = "Falha na verificação: " + err.Error()
			continue
		}

		resultadosFinais[alvo.Nome] = relatorio
		registros[alvo.URL] = registroAtualizado
		log.Printf("--- Verificação concluída para: %s ---", alvo.Nome)
	}

	if err := persistence.SalvarRegistrosJSON(registros); err != nil {
		log.Fatalf("ERRO CRÍTICO: Falha ao salvar o banco de dados JSON: %v", err)
	}

	log.Println("\n--- Todas as verificações foram concluídas com sucesso ---")
	log.Println("Enviando e-mail com o relatório final...")
	if err := email.EnviarEmailRelatorioFinal(cfg, resultadosFinais); err != nil {
		log.Printf("ERRO CRÍTICO: Falha ao enviar o e-mail: %v", err)
	} else {
		log.Println("E-mail de relatório final enviado com sucesso.")
	}
}
