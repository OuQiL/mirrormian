// .env 文件读写：设置页持久化配置用（合并写，保留既有内容）。
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// ReadEnvFile 读取 .env 为 map；文件不存在时返回空 map（非错误）。
func ReadEnvFile(path string) (map[string]string, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	env, err := godotenv.Read(path)
	if err != nil {
		return nil, fmt.Errorf("读取 %s: %w", path, err)
	}
	return env, nil
}

// WriteEnvFile 写回 .env（godotenv.Write 保留注释之外的全部 key）。
func WriteEnvFile(path string, env map[string]string) error {
	if err := godotenv.Write(env, path); err != nil {
		return fmt.Errorf("写入 %s: %w", path, err)
	}
	return nil
}
