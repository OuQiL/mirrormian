.PHONY: run train review profile build test clean

# 运行健康自检
run:
	go run ./cmd run

# 专项面试：make train ARGS="special Redis"
train:
	go run ./cmd train $(ARGS)

# 查询/提交到期复习：make review ARGS="--score <id> 8"
review:
	go run ./cmd review $(ARGS)

# 查看画像
profile:
	go run ./cmd profile

# 编译
build:
	go build -o bin/mian.exe ./cmd

# 运行测试
test:
	go test ./... -v

# 清理编译产物
clean:
	rm -rf bin/
