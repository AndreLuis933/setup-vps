# data/scripts

Este diretório contém os scripts mais importantes do projeto. Eles são usados no
bootstrap do ambiente, no deploy automatizado e em tarefas de renovação de
certificados/NGINX.

## Visão geral rápida

- `bootstrap`: inicializa o ambiente e gera certificados (dev ou prod).
- `deploy`: faz o deploy (pull da imagem, sobe containers e limpa imagens
  antigas).
- `watcher`: observa a fila de webhooks e dispara o `deploy`.
- `create_nginx_conf`: gera o arquivo final do NGINX a partir de templates.
- `certbot_create_prod_certs`: cria certificados reais (apenas em production).
- `certbot_renewal`: loop de renovação do certbot dentro do container.
- `nginx_renewal`: loop de reload do NGINX dentro do container.
- `backup_volumes`: backup do volume compartilhado (`data_vol`).
- `entrypoint`: inicia o servidor FastAPI (Uvicorn) no container app.
- `setup_user`: ajustes de shell (vi-mode, editor padrão, atalho).

## Pré-requisitos gerais

- Docker + Docker Compose funcionando no servidor.
- `.env` configurado na raiz do projeto.
- Containers definidos no `compose.yaml`.

## Scripts (um a um)

### `bootstrap`

**Quando usar:**

- Primeira subida do projeto (development e depois production).
- Quando precisar regenerar certificados e reconfigurar o NGINX.

Importante: em `production`, ele gera os certificados SSL todas as vezes que
você o executar. O ideal é executar ele uma vez apenas no bootstrap da aplicação
(na inicialização). Já em `development` você pode rodar ele quantas vezes
quiser.

**O que faz:**

1. Sobe os containers base.
2. Gera certificados de desenvolvimento.
3. Em `production`, cria o NGINX temporário de HTTP, roda o certbot e valida.
4. Gera o NGINX final (HTTPS) e sobe tudo novamente.
5. Faz backup antes e depois.

**Pontos de atenção:**

- Requer `.env` valido e `CURRENT_ENV` definido.
- Em production, depende de `DOMAINS` e `EMAIL` corretos.

**Uso:**

```sh
/data/scripts/bootstrap
```

---

### `deploy`

**Quando usar:**

- Deploy manual (ou chamado pelo `watcher`).

**O que faz:**

1. Faz backup do volume.
2. Em production, sincroniza o repo com `origin/$DEPLOY_BRANCH`.
3. Copia `data/` para o volume compartilhado.
4. `docker compose pull` + `up -d`.
5. Limpa imagens antigas.
6. Faz backup final.

**Pontos de atenção:**

- Em production, usa `git reset --hard` para alinhar com o remoto.
- Depende de `.env` (principalmente `CURRENT_ENV` e `DEPLOY_BRANCH`).

**Uso:**

```sh
/data/scripts/deploy
```

---

### `watcher`

**Quando usar:**

- Rodando como serviço systemd no host.

**O que faz:**

- Verifica a cada 60s se existem arquivos em `/data/webhook_jobs`.
- Se houver, faz debounce (apaga todos) e roda um único `deploy`.
- Usa `flock` para garantir um deploy por vez.

**Pontos de atenção:**

- O diretório de jobs esta dentro do volume do container `data_vol`.
- O lock file vive no host: `/tmp/webhook_deploy.lock`.

**Uso (systemd):** Veja o DEV_GUIDE.md para o service `webhook-watcher`.

---

### `create_nginx_conf`

**Quando usar:**

- Gerado automaticamente pelo `bootstrap`.
- Manualmente, se você editar templates.

**O que faz:**

- Usa `envsubst` para gerar o `app.conf` a partir de templates.
- Em `development`, troca `DOMAINS` por `_` (host genérico).

**Pontos de atenção:**

- Requer `DOMAINS` e `CURRENT_ENV` no ambiente.

**Uso (dentro do container):**

```sh
/data/scripts/create_nginx_conf app.conf.template
```

---

### `certbot_create_prod_certs`

**Quando usar:**

- Apenas em `production` e no bootstrap.

**O que faz:**

- Roda `certbot certonly` com webroot e todos os domínios.

**Pontos de atenção:**

- Falha se `CURRENT_ENV` não for `production`.
- Depende de `DOMAINS` e `EMAIL`.

---

### `certbot_renewal`

**Quando usar:**

- Rodando dentro do container certbot.

**O que faz:**

- Loop que tenta renovar a cada 12h.

**Pontos de atenção:**

- O comportamento e controlado por variáveis internas do script.

---

### `nginx_renewal`

**Quando usar:**

- Rodando dentro do container nginx.

**O que faz:**

- Sobe o nginx em foreground e faz reload a cada 6h.

**Pontos de atenção:**

- Garante que novas chaves SSL sejam recarregadas.

---

### `backup_volumes`

**Quando usar:**

- Chamado automaticamente por `bootstrap` e `deploy`.
- Pode ser executado manualmente.

**O que faz:**

- Faz `docker compose cp` do volume do `data_vol`.
- Compacta em `backups/` com timestamp.

**Uso:**

```sh
/data/scripts/backup_volumes
```

---

### `entrypoint`

**Quando usar:**

- Usado pela imagem do app.

**O que faz:**

- Sobe Uvicorn com configuração otimizada.

---

### `setup_user`

**Quando usar:**

- Opcional, apenas para ajustar o shell do usuário no servidor.

**O que faz:**

- Habilita vi-mode e configura editor.

## Variaveis de ambiente importantes

Estas variáveis vivem no `.env` da raiz do projeto:

- `CURRENT_ENV`: `development` ou `production`.
- `DOMAINS`: lista de domínios (separados por espaço).
- `EMAIL`: email para o certbot.
- `DEPLOY_BRANCH`: branch usado no deploy em production.

## Dicas para iniciantes

- Sempre confira o `.env` antes de rodar `bootstrap` ou `deploy`.
- Em production, o `deploy` faz `git reset --hard` no branch remoto.
- Em caso de erro, verifique:
  - `docker compose ps`
  - logs do `webhook-watcher` via `journalctl`
  - arquivos em `backups/` para rollback de dados

## Onde aprender mais

- `AGENTS.md` (visão do projeto)
- `DEV_GUIDE.md` (passo a passo do video)
