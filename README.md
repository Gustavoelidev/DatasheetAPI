# Monitor de Datasheet e Conteúdo Web

![Go Version](https://img.shields.io/badge/Go-1.18%2B-blue.svg)
![License](https://img.shields.io/badge/License-MIT-green.svg)

Um monitor automatizado escrito em Go que vigia páginas web para detectar alterações em arquivos (como Datasheets em PDF) e seções de especificações HTML, enviando notificações por e-mail com um relatório consolidado.

## Objetivo

Este projeto foi criado para resolver a necessidade de acompanhar atualizações em páginas de produtos que nem sempre oferecem um sistema de notificação. Ele automatiza o processo de verificação manual, garantindo que qualquer mudança em documentos importantes ou especificações técnicas seja detectada e reportada rapidamente.

## Funcionalidades Principais

* **Monitoramento de Múltiplos Alvos:** Configure facilmente uma lista de URLs para monitorar através de um arquivo `config.json`.
* **Detecção de Alteração de Arquivos:** Compara o hash MD5 de arquivos para download (ex: Datasheets) para detectar qualquer modificação.
* **Detecção de Alteração de Conteúdo:** Compara o hash MD5 de seções HTML específicas (ex: abas de "Especificações") para identificar mudanças no conteúdo da página.
* **Relatórios de Diferença (Diff):** Quando uma alteração em HTML é detectada, um arquivo `diff.html` é gerado, destacando visualmente as linhas que foram adicionadas ou removidas.
* **Notificações por E-mail:** Envia um único relatório consolidado por e-mail ao final de cada execução, resumindo o status de todos os alvos (sem alterações, alterações detectadas ou falhas).
* **Armazenamento de Histórico:** Mantém um registro local (`monitor_dados_geral.json`) do estado da última verificação para comparação futura.

## Tecnologias Utilizadas

* **[Go](https://golang.org/)**: Linguagem principal da aplicação.
* **[gopkg.in/gomail.v2](https://github.com/go-gomail/gomail)**: Para o envio de e-mails via SMTP.
* **[github.com/PuerkitoBio/goquery](https://github.com/PuerkitoBio/goquery)**: Para fazer o web scraping e parsing do HTML.
* **[github.com/sergi/go-diff](https://github.com/sergi/go-diff)**: Para gerar os relatórios de diferenças entre os conteúdos HTML.

## Configuração

Antes de rodar a aplicação, é necessário configurar suas credenciais e os alvos de monitoramento.

1.  Renomeie o arquivo `config.example.json` para `config.json` (ou crie um novo).
2.  Preencha com as suas informações:

```json
{
  "SmtpHost": "smtp.seuprovedor.com",
  "SmtpPorta": 587,
  "SmtpUsuario": "seu-email@provedor.com",
  "SmtpSenha": "sua-senha-de-app",
  "EmailDestinatario": [
    "destinatario1@email.com",
    "destinatario2@email.com"
  ],
  "Alvos": [
    {
      "Nome": "Nome do Produto 1",
      "URL": "[https://www.site.com/produto1](https://www.site.com/produto1)"
    },
    {
      "Nome": "Nome do Produto 2",
      "URL": "[https://www.site.com/produto2](https://www.site.com/produto2)"
    }
  ]
}
```
* **`SmtpSenha`**: É altamente recomendado usar uma "Senha de App" se o seu provedor de e-mail (como o Gmail) oferecer essa opção, em vez de sua senha principal.

## Como Rodar

1.  **Clone o repositório:**
    ```bash
    git clone [https://github.com/seu-usuario/datasheet-monitor.git](https://github.com/seu-usuario/datasheet-monitor.git)
    cd datasheet-monitor
    ```

2.  **Crie e preencha o arquivo `config.json`** conforme a seção de configuração acima.

3.  **Instale as dependências:**
    ```bash
    go mod tidy
    ```

4.  **Execute a aplicação:**
    ```bash
    go run cmd/monitor/main.go
    ```

5.  **(Opcional) Compile para um executável:**
    ```bash
    go build -o monitor cmd/monitor/main.go
    ```
    E depois rode o executável:
    ```bash
    ./monitor
    ```

### Agendando a Execução (Exemplo com Cron no Linux)

Para rodar o monitor automaticamente todos os dias às 08:00, você pode usar o Cron:
```bash
# Edite o crontab
crontab -e

# Adicione a linha abaixo, ajustando o caminho para o seu executável
0 8 * * * /caminho/completo/para/o/projeto/monitor >> /caminho/para/log.txt 2>&1
```

## Estrutura do Projeto

O projeto segue uma estrutura de Clean Architecture para separar as responsabilidades:

```
monitor-datasheet/
├── cmd/monitor/main.go         # Ponto de entrada da aplicação
├── internal/
│   ├── config/                 # Lógica para carregar config.json
│   ├── core/
│   │   ├── domain/             # Structs principais (Alvo, Config, etc.)
│   │   └── services/           # Lógica de negócio principal
│   ├── infrastructure/
│   │   ├── email/              # Lógica de envio de email
│   │   ├── persistence/        # Lógica para ler/salvar o JSON de estado
│   │   └── scraper/            # Lógica de web scraping
│   └── pkg/utils/              # Funções auxiliares (hash, diff, etc.)
├── go.mod
└── config.json
```

## Licença

Este projeto está sob a licença MIT. Veja o arquivo [LICENSE](LICENSE) para mais detalhes.

---
