####################MAKEFILE########################
# 			Makefile for crmintratools			   #
####################################################


#main bins
GO=go
GOFMT=gofmt
BKP_CMD=tar -czvf 

#env
THIS_DIR := $(CURDIR)
TIMESTAMP=$(shell date +%Y%m%d%H%M%S)
APP_VERSION=$(TIMESTAMP)
VERSION_FILE=goworkers/version.go

#out
BKP_DIR=Backups
OUT_BKP_NAME=$(THIS_DIR)/$(BKP_DIR)/crmintratools_backup-$(TIMESTAMP).tgz
OUT_BIN_NAME=crmintratools.bin

#info cmd's
.PHONY: all build compile clean run format backup depends count version

#default cmd
all:
	@echo "Las opciones son: run,build,compile,clean,format,backup,depends"

version:
	@echo "Generando version frontend: $(APP_VERSION)"
	@echo 'package goworkers' > $(VERSION_FILE)
	@echo '' >> $(VERSION_FILE)
	@echo 'const AppVersion = "$(APP_VERSION)"' >> $(VERSION_FILE)

#available cmd's
run: version
	$(GO) run $(THIS_DIR)
	
build: version
	$(GO) build -o $(OUT_BIN_NAME) $(THIS_DIR)
	@echo "Compilado creado exitosamente"

compile: version
	@echo "Compilando modulos en Modules/* ..."
	@for dir in Modules/*; do \
		if [ -d "$$dir" ] && [ -f "$$dir/main.go" ]; then \
			modname=$$(basename $$dir); \
			echo "→ Compilando $$modname ..."; \
			( cd "$$dir" && go build -o "$$modname.bin" main.go ) || echo "Fallo al compilar $$modname"; \
		fi; \
	done
	@echo "Compilacion de todos los modulos finalizada"

clean:
	$(GO) clean $(THIS_DIR)
	rm -f $(OUT_BIN_NAME)
	rm -f $(VERSION_FILE)
	@echo "Binario principal eliminado: $(OUT_BIN_NAME)"
	@echo "Eliminando binarios de modulos..."
	@find Modules -type f -name "*.bin" -exec rm -f {} \;
	@echo "Limpieza completada"

format:
	$(GOFMT) $(THIS_DIR)
	@echo "Codigo formateado exitosamente"
backup:
	@mkdir -p $(BKP_DIR)
	$(BKP_CMD) $(OUT_BKP_NAME) --exclude=$(THIS_DIR)/$(BKP_DIR)/'*' $(THIS_DIR)
	@echo "Backup creado: $(OUT_BKP_NAME)"
depends:
	$(GO) mod tidy
	@echo "Dependencias satisfechas exitosamente"
count:
	@echo "Contando lineas de archivos .go:"
	@find . -name '*.go' -type f -print0 | xargs -0 wc -l | sort -n > .go_linecount.tmp
	@cat .go_linecount.tmp | grep -v total
	@GO_TOTAL=$$(tail -n1 .go_linecount.tmp | awk '{print $$1}'); \
	echo "Total lineas de codigo Go: $$GO_TOTAL"
	@rm -f .go_linecount.tmp

	@echo "\nContando lineas de archivos .html:"
	@find . -name '*.html' -type f -print0 | xargs -0 wc -l | sort -n > .html_linecount.tmp
	@cat .html_linecount.tmp | grep -v total
	@HTML_TOTAL=$$(tail -n1 .html_linecount.tmp | awk '{print $$1}'); \
	echo "Total lineas de codigo HTML: $$HTML_TOTAL"
	@rm -f .html_linecount.tmp


