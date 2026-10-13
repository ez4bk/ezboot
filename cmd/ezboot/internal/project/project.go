package project

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	DefaultRepo    = "https://github.com/ez4bk/ezboot-template"
	DefaultTimeout = 180 * time.Second

	// dbBlockBegin/dbBlockEnd 是模板中数据库条件块的标记。
	// 标记行本身在生成时总会被移除, 块内内容仅在启用 --with-db 时保留。
	dbBlockBegin = "ezboot:with-db:begin"
	dbBlockEnd   = "ezboot:with-db:end"
)

// dbOnlyPaths 是仅在启用 --with-db 时才生成的模板路径(相对模板仓库根目录)
var dbOnlyPaths = []string{
	"dbgen/",
	"schemas/",
	"internal/dao/",
}

// CommandNew 创建一个新项目
func CommandNew() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "create a service project using the repository template",
		PreRun: func(cmd *cobra.Command, args []string) {
			if len(options.repo) == 0 {
				options.repo = DefaultRepo
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), options.timeout)
			defer cancel()

			var projectName string
			if len(args) == 0 {
				prompt := &survey.Input{
					Message: "What is project name ?",
				}

				err := survey.AskOne(prompt, &projectName)
				if err != nil || len(projectName) == 0 {
					return nil
				}
			} else {
				projectName = args[0]
			}

			if len(options.module) == 0 {
				prompt := &survey.Input{
					Message: "What is golang module name (without project name) ?",
				}

				_ = survey.AskOne(prompt, &options.module)
			}

			if len(options.module) != 0 {
				options.module = strings.TrimRight(options.module, "/")
				options.module += "/"
			}

			return createProject(ctx, projectName)
		},
	}

	cmd.PersistentFlags().StringVar(&options.repo, "repo", "", "the address of the service template")
	cmd.PersistentFlags().StringVar(&options.module, "module", "", "the name of the golang project")
	cmd.PersistentFlags().DurationVar(&options.timeout, "timeout", DefaultTimeout, "timeout for creates a project")
	cmd.PersistentFlags().BoolVar(&options.withoutGit, "without-git", false, "without git-vcs init")
	cmd.PersistentFlags().BoolVar(&options.withDB, "with-db", false, "generate database schemas, dbgen and mysql configuration")

	return cmd
}

// options 创建新项目所需要使用的参数
var options struct {
	repo       string        // 模板仓库的地址
	module     string        // Go项目模板
	timeout    time.Duration // 创建项目超时时间
	withoutGit bool          // 是否要生成Git仓库
	withDB     bool          // 是否要生成数据库相关的内容
}

// createProject 根据指定的参数创建并生成一个项目
func createProject(ctx context.Context, name string) (err error) {
	projectDir, err := filepath.Abs(name)
	if err != nil {
		return err
	}

	stat, err := os.Stat(projectDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if stat != nil && !stat.IsDir() {
		return errors.Errorf("the project(%s) already exists", name)
	} else if stat != nil && stat.IsDir() {
		dir, err := os.Open(projectDir)
		if err != nil {
			return err
		}
		defer func() { _ = dir.Close() }()

		fis, err := dir.Readdir(1)
		if err != nil {
			return err
		}

		if len(fis) != 0 {
			return errors.Errorf("the project(%s) not empty", name)
		}
	} else if stat == nil || os.IsNotExist(err) {
		if err = os.MkdirAll(projectDir, fs.FileMode(0755)); err != nil {
			return err
		}
	}

	defer func() {
		if err != nil {
			_ = os.RemoveAll(projectDir)
		}
	}()

	fmt.Printf("🚀 Creating service %s, please wait a moment.\n", name)
	for file := range clone(ctx, options.repo) {
		if file.Error != nil {
			return file.Error
		}

		if !options.withDB && isDBOnlyPath(file.RelPath) {
			continue
		}

		err = renderAndWrite(filepath.Join(projectDir, file.RelPath), file.Content, buildParams(name), options.withDB)
		if err != nil {
			return err
		}
	}

	if !options.withoutGit {
		if err = initRepo(projectDir); err != nil {
			return err
		}
	}

	fmt.Printf("\n🍺 Project creation succeeded %s\n", color.GreenString(name))
	return nil
}

// renderAndWrite 渲染并生成对应的项目目录结构
func renderAndWrite(dst string, src io.ReadCloser, params map[string]string, withDB bool) error {
	// do not forget close file
	defer func() { _ = src.Close() }()

	data, err := ioutil.ReadAll(src)
	if err != nil {
		return err
	}

	dst = string(render([]byte(dst), params))
	if err = os.MkdirAll(filepath.Dir(dst), fs.FileMode(0755)); err != nil {
		return err
	}

	fp, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = fp.Close() }()

	if isBinary(data) {
		_, err = fp.Write(data)
		return err
	}

	_, err = fp.Write(render(pruneDBBlocks(data, withDB), params))
	return err
}

// pruneDBBlocks 移除模板中的数据库条件块标记
// 标记行本身始终会被移除, 块内内容仅在 withDB 为 true 时保留
func pruneDBBlocks(data []byte, withDB bool) []byte {
	if !bytes.Contains(data, []byte(dbBlockBegin)) {
		return data
	}

	lines := bytes.SplitAfter(data, []byte("\n"))
	result := make([]byte, 0, len(data))
	depth := 0
	for _, line := range lines {
		switch {
		case bytes.Contains(line, []byte(dbBlockBegin)):
			depth++
		case bytes.Contains(line, []byte(dbBlockEnd)):
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 || withDB {
				result = append(result, line...)
			}
		}
	}

	return result
}

// isDBOnlyPath 判断模板文件是否仅在启用 --with-db 时才需要生成
func isDBOnlyPath(relPath string) bool {
	for _, prefix := range dbOnlyPaths {
		if strings.HasPrefix(relPath, prefix) {
			return true
		}
	}

	return false
}

// isBinary 检查文件是否是二进制文件
func isBinary(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

// render 渲染特定的内容
func render(data []byte, params map[string]string) []byte {
	for k, v := range params {
		data = bytes.ReplaceAll(data, []byte(k), []byte(v))
	}
	return data
}

// buildParams 构建模板参数
func buildParams(name string) map[string]string {
	return map[string]string{
		"myna":            name,
		"example.ezboot/": options.module,
	}
}
