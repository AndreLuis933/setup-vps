# Diretório `data`

Este diretório contém todos os arquivos de configuração e scripts essenciais que são usados pelos contêineres em produção, mas que **não são** parte do código-fonte da aplicação principal.

Isso inclui:
- Templates de configuração do NGINX (`templates/`).
- Scripts de inicialização, deploy e automação (`scripts/`).
- Configurações estáticas do NGINX (`nginx.conf`, `mime.types`, etc.).
- Arquivos de certificados SSL do Let's Encrypt (gerados e armazenados aqui).

## Arquitetura: Por que não usamos `Bind Mounts`?

Você notará no `compose.yaml` que não usamos `bind mounts` diretos do host para este diretório, como é comum em muitos projetos Docker. Em vez disso, o conteúdo desta pasta é copiado para dentro de uma imagem dedicada chamada `data_vol` durante o processo de `build`.

**O motivo para este design é suportar ambientes de desenvolvimento remoto flexíveis.**

Este projeto foi desenvolvido em um cenário onde o cliente Docker (a linha de comando) e o motor Docker (o daemon) rodam em máquinas diferentes. Em tal configuração, `bind mounts` diretos falham, pois o motor Docker não tem acesso ao sistema de arquivos do cliente.

Ao copiar os arquivos para uma imagem, garantimos que todas as configurações e scripts viajem junto com o "build context" para o motor Docker, tornando o setup independente da localização física dos arquivos. Os outros serviços, como `nginx`, acessam esses dados usando a diretiva `volumes_from: [data_vol]`.
