# ========================
# Config
# ========================

BIN_DIR := bin

# List your binaries here
BINARIES := amaros cron llm_execute_memory llm_execute node_proxy telegram console

# Map binaries -> source files
amaros_SRC := cmd/main.go
cron_SRC  := examples/cron/*
llm_execute_memory_SRC   := examples/llm_execute_memory/*
llm_execute_SRC   := examples/llm_execute/*
node_proxy_SRC := examples/node_proxy/*
telegram_SRC := examples/messaging/telegram/*
console_SRC := examples/messaging/console/*

# Detect shell config file
SHELL_TYPE := $(HOME)/.bashrc

ifeq ($(SHELL),/bin/zsh)
	SHELL_TYPE := $(HOME)/.zshrc
endif
ifeq ($(SHELL),/bin/fish)
	SHELL_TYPE := $(HOME)/.config/fish/config.fish
endif

# ========================
# PATH export helper
# ========================

define check_in_path
	if ! echo "$(PATH)" | grep -q "$(PWD)/$(BIN_DIR)"; then \
		echo "Exporting $(BIN_DIR) to PATH in $(SHELL_TYPE)"; \
		echo 'export PATH=$(PWD)/$(BIN_DIR):$$PATH' >> $(SHELL_TYPE); \
		. $(SHELL_TYPE); \
	else \
		echo "$(PWD)/$(BIN_DIR) is already in PATH."; \
	fi
endef

# ========================
# Targets
# ========================

ALL_BINS := $(addprefix $(BIN_DIR)/,$(BINARIES))

all: build export

build: $(ALL_BINS)

# Generic build rule
$(BIN_DIR)/%:
	@echo "Building $*..."
	@go build -o $(BIN_DIR)/$* $($*_SRC)

# Run targets (auto-generated style)
run-%: $(BIN_DIR)/%
	@echo "Running $*..."
	@$(BIN_DIR)/$*

# Clean
clean:
	@echo "Cleaning binaries..."
	@rm -f $(ALL_BINS)

export:
	@$(call check_in_path)

.PHONY: all build clean export $(addprefix run-,$(BINARIES))