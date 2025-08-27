#!/bin/bash

# Auto-generate gomock files to avoid manual mock maintenance
# Usage: ./scripts/generate_mocks.sh [filename...]

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

generated_count=0

# Create mock directories
mkdir -p tests/mock/usecase
mkdir -p tests/mock/commands
mkdir -p tests/mock/queries
mkdir -p tests/mock/repository
mkdir -p tests/mock/readstore

# Function to generate mock for a specific file
generate_mock() {
    local source_file="$1"
    local mock_dir="$2"
    local mock_package="$3"
    
    [ -f "$source_file" ] || return
    
    filename=$(basename "$source_file" .go)
    
    # Check if file contains interfaces
    if grep -q "type.*interface" "$source_file"; then
        echo -e "${GREEN}Generating mock: ${source_file} -> ${mock_dir}${NC}"
        mockgen -source="$source_file" \
                -destination="${mock_dir}/${filename}_mock.go" \
                -package="$mock_package"
        generated_count=$((generated_count + 1))
    fi
}

# Generate mocks for different directories
echo "Generating mocks for usecase layer..."

# TokenValidator (usecase root)
generate_mock "internal/usecase/token_validator.go" "tests/mock/usecase" "usecasemock"

# Commands
for file in internal/usecase/commands/*.go; do
    generate_mock "$file" "tests/mock/commands" "commandsmock"
done

# Queries  
for file in internal/usecase/queries/*.go; do
    generate_mock "$file" "tests/mock/queries" "queriesmock"
done

# Repository (infrastructure layer)
echo "Generating mocks for repository layer..."
for file in internal/infra/repository/*.go; do
    generate_mock "$file" "tests/mock/repository" "repositorymock"
done

# ReadStore (infrastructure layer)
for file in internal/infra/readstore/*.go; do
    generate_mock "$file" "tests/mock/readstore" "readstoremock"
done

if [ $generated_count -eq 0 ]; then
    echo -e "${RED}No interfaces found${NC}"
    exit 1
fi

echo -e "${GREEN}Generated $generated_count mock files${NC}"
