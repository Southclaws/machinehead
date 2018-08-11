package gitwatch_test

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"github.com/Southclaws/machinehead/gitwatch"
	"github.com/bmizerany/assert"
)

var (
	gw      *gitwatch.Session
	ctx     context.Context
	cf      context.CancelFunc
	initial time.Time
)

func TestMain(m *testing.M) {
	var err error

	mockRepo("a")
	mockRepo("b")

	ctx, cf = context.WithCancel(context.Background())
	gw, err = gitwatch.New(
		ctx,
		[]string{"./test/local/a", "./test/local/b"},
		time.Second,
		"./test/",
		true,
	)
	if err != nil {
		panic(err)
	}

	// consume clone events
	<-gw.Events
	<-gw.Events

	ret := m.Run()

	gw.Close()

	os.Exit(ret)
}

func TestMakeChange1(t *testing.T) {
	ts := mockRepoChange("a", "hello world!")

	event := <-gw.Events
	assert.Equal(t, event, gitwatch.Event{
		URL:       "./test/local/a",
		Path:      fullPath("./test/a"),
		Timestamp: ts.Truncate(time.Second),
	})
}

func TestMakeChange2(t *testing.T) {
	tsa := mockRepoChange("a", "hello world!!")
	tsb := mockRepoChange("b", "hello earth")

	event := <-gw.Events
	assert.Equal(t, gitwatch.Event{
		URL:       "./test/local/a",
		Path:      fullPath("./test/a"),
		Timestamp: tsa.Truncate(time.Second),
	}, event)

	event = <-gw.Events
	assert.Equal(t, gitwatch.Event{
		URL:       "./test/local/b",
		Path:      fullPath("./test/b"),
		Timestamp: tsb.Truncate(time.Second),
	}, event)
}

func mockRepo(name string) {
	dirPath := filepath.Join("./test/local/", name)
	err := os.RemoveAll(dirPath)
	if err != nil {
		panic(err)
	}
	err = os.RemoveAll(filepath.Join("./test", name))
	if err != nil {
		panic(err)
	}
	repo, err := git.PlainInit(dirPath, false)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(filepath.Join(dirPath, "file"), []byte("hello world"), 0666)
	if err != nil {
		panic(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		panic(err)
	}
	_, err = wt.Add("file")
	if err != nil {
		panic(err)
	}
	_, err = wt.Commit("first", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		panic(err)
	}
}

func mockRepoChange(name, contents string) time.Time {
	dirPath := filepath.Join("./test/local/", name)
	repo, err := git.PlainOpen(dirPath)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(filepath.Join(dirPath, "file"), []byte(contents), 0666)
	if err != nil {
		panic(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		panic(err)
	}
	_, err = wt.Add("file")
	if err != nil {
		panic(err)
	}
	ts := time.Now()
	_, err = wt.Commit("add: "+contents, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@test.com",
			When:  ts,
		},
	})
	if err != nil {
		panic(err)
	}
	return ts
}

func fullPath(relative string) (result string) {
	result, err := filepath.Abs(relative)
	if err != nil {
		panic(err)
	}
	return
}
