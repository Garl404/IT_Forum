.PHONY: build run test vet fmt check clean

# 编译后端
build:
	cd backend && go build -o caluse-server ./cmd/server

# 本地运行（前台）
run:
	cd backend && go run ./cmd/server -config config.yaml

# 单元测试
test:
	cd backend && go test ./...

# 静态检查
vet:
	cd backend && go vet ./...

# 格式化
fmt:
	cd backend && gofmt -l -w .

# 全量检查（CI 同款）
check:
	cd backend && go build ./... && go vet ./... && go test ./...
	for f in frontend/js/*.js; do node --check "$$f" || exit 1; done
	sh -n scripts/caluse.sh
	@echo "全部检查通过 ✓"

# 清理编译产物
clean:
	rm -f backend/caluse-server
