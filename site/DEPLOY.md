# Como publicar o site no GitHub Pages

Este guia explica como hospedar o site estático do ZapLab gratuitamente no seu domínio `github.io`.

## Passo 1: Commit dos arquivos
Certifique-se de que a pasta `site/` está no seu repositório Git e faça o push para o GitHub:
```bash
git add site/
git commit -m "docs: add static landing page"
git push origin main
```

## Passo 2: Configuração no GitHub
1. Vá para o repositório **[zaplab](https://github.com/lichti/zaplab)** no GitHub.
2. Clique na aba **Settings** (Configurações).
3. No menu lateral esquerdo, clique em **Pages**.
4. Em **Build and deployment** > **Source**, selecione "Deploy from a branch".
5. Em **Branch**, selecione `main` (ou a sua branch principal) e mude a pasta de `/ (root)` para `/site`.
6. Clique em **Save**.

## Passo 3: Verificação
O GitHub levará alguns minutos para processar os arquivos. Após a conclusão, você verá uma mensagem no topo da página de configurações com o link:
`Your site is live at https://lichti.github.io/zaplab/`

---

## Dicas Extras

### Domínio Customizado
Se você tiver um domínio próprio, pode configurá-lo na mesma página de configurações do GitHub Pages em "Custom domain".

### Assets (Imagens)
As imagens usadas no site foram copiadas para `site/images/`. Se você adicionar novas capturas de tela em `docs/images/`, lembre-se de copiá-las também para a pasta `site/images/` para que fiquem visíveis no site estático.
