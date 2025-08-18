#!/bin/bash

# Auto-generate gomock files to avoid manual mock maintenance
# Usage: ./scripts/generate_mocks.sh [filename...]

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

mkdir -p tests/mock/usecase
generated_count=0

# Process specific files or all usecase files
if [ $# -eq 0 ]; then
    files="internal/usecase/*.go"
else
    files=""
    for arg in "$@"; do
        files="$files internal/usecase/${arg}.go"
    done
fi

for file in $files; do
    [ -f "$file" ] || continue
    
    filename=$(basename "$file" .go)
    
    if grep -q "type.*UseCase interface" "$file"; then
        echo -e "${GREEN}Generating mock: ${filename}.go${NC}"
        mockgen -source="$file" \
                -destination="tests/mock/usecase/${filename}_mock.go" \
                -package=usecasemock
        generated_count=$((generated_count + 1))
    fi
done

if [ $generated_count -eq 0 ]; then
    echo -e "${RED}No UseCase interfaces found${NC}"
    exit 1
fi