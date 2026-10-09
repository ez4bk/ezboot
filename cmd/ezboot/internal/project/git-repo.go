package project

import (
	"context"
	"io"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pkg/errors"
)

// vcsFile 表示一个受版本管理的文件
type vcsFile struct {
	RelPath string
	Content io.ReadCloser

	Error error
}

// clone 从指定的位置克隆项目
func clone(ctx context.Context, url string) chan *vcsFile {
	ch := make(chan *vcsFile, 1)
	go func() {
		defer close(ch)

		repo, err := git.CloneContext(ctx, memory.NewStorage(), nil, &git.CloneOptions{
			URL:      url,
			Depth:    1,
			Progress: io.Discard,
		})
		if err != nil {
			ch <- &vcsFile{Error: errors.Wrap(err, "git")}
			return
		}

		if err = walkTree(ctx, repo, ch); err != nil {
			ch <- &vcsFile{Error: errors.Wrap(err, "git")}
			return
		}
	}()
	return ch
}

// walkTree 遍历仓库的文件树
func walkTree(ctx context.Context, repo *git.Repository, ch chan<- *vcsFile) error {
	head, err := repo.Head()
	if err != nil {
		return errors.Wrap(err, "git")
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return errors.Wrap(err, "git")
	}

	tree, err := commit.Tree()
	if err != nil {
		return errors.Wrap(err, "git")
	}

	return tree.Files().ForEach(func(f *object.File) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			reader, err := f.Reader()
			if err != nil {
				return errors.Wrap(err, "git")
			}

			ch <- &vcsFile{RelPath: f.Name, Content: reader}
			return nil
		}
	})
}

// initRepo 初始化仓库并提交第一个内容
func initRepo(path string) error {
	repo, err := git.PlainInit(path, false)
	if err != nil {
		return errors.Wrap(err, "git")
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return errors.Wrap(err, "git")
	}

	err = worktree.AddWithOptions(&git.AddOptions{All: true})
	if err != nil {
		return errors.Wrap(err, "git")
	}

	_, err = worktree.Commit("initial commit", &git.CommitOptions{})
	if err != nil {
		return errors.Wrap(err, "git")
	}

	return nil
}
