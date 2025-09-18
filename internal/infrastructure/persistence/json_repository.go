package persistence

import (
	"datasheetApi/internal/core/domain"
	"encoding/json"
	"log"
	"os"
)

const ARQUIVO_JSON_DADOS = "monitor_dados_geral.json"

func CarregarRegistrosJSON() (domain.BancoDeDados, error) {
	registros := make(domain.BancoDeDados)
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

func SalvarRegistrosJSON(registros domain.BancoDeDados) error {
	dados, err := json.MarshalIndent(registros, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ARQUIVO_JSON_DADOS, dados, 0644)
}
