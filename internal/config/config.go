package config

import (
	"datasheetApi/internal/core/domain"
	"encoding/json"
	"os"
)

func CarregarConfiguracao(caminho string) (domain.Config, error) {
	var config domain.Config
	arquivo, err := os.ReadFile(caminho)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(arquivo, &config)
	return config, err
}
