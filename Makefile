# Clinic Makefile

# Generates server files
generate:
	cmd/regenerateYaml.sh

# Runs tests
test:
	./test.sh

# Builds package
build:
	./build.sh
