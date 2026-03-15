# Define binary output paths
ROS_CORE_BIN := bin/
SHELL_TYPE := $(HOME)/.bashrc  # Default to bashrc, will be overridden based on SHELL variable

# get shell type
ifeq ($(SHELL),/bin/bash)
	SHELL_TYPE := $(HOME)/.bashrc
endif
ifeq ($(SHELL),/bin/zsh)
	SHELL_TYPE := $(HOME)/.zshrc
endif
ifeq ($(SHELL),/bin/fish)
	SHELL_TYPE := $(HOME)/.config/fish/config.fish
endif

# Define source files
ROS_CORE_SRC := cmd/main.go examples/messaging/console_messaging examples/messaging/telegram_messaging examples/llm_inference

# Function to check if the directory is in PATH
define check_in_path
	if ! echo "$(PATH)" | grep -q "$(PWD)/bin"; then \
		echo "Exporting amaros_BIN to PATH in $(SHELL_TYPE)"; \
		echo 'export PATH=$(PWD)/bin:$$PATH' >> $(SHELL_TYPE); \
		. $(SHELL_TYPE) \
	else \
		echo "$(PWD)/bin is already in PATH."; \
	fi
endef


# Default build command
all: build export

# Build the ros_core and topic binaries
build: $(ROS_CORE_BIN)

$(ROS_CORE_BIN): $(ROS_CORE_SRC)
	@echo "Building AMAROS binary..."
	@go build -o $(ROS_CORE_BIN) $(ROS_CORE_SRC)
	chmod u+s $(ROS_CORE_BIN) 


# Run the ros_core binary
ros_core: $(ROS_CORE_BIN)
	@echo "Running AMAROS binary..."
	@$(ROS_CORE_BIN)

# Clean up build artifacts
clean:
	@echo "Cleaning up binaries..."
	@rm -f $(ROS_CORE_BIN)/*

export:
	@$(call check_in_path)

.PHONY: all build ros_core topic clean
