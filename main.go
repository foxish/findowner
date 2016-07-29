package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"io"
	"path"
)

var (
	githubToken string
	githubOrg   string
	githubRepo  string
	topLevelDir string
	localRepo   string
	depth       int
)

var now time.Time

type commitWeight struct {
	Rank  float64
	Lines float64
}

func ExitError(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func init() {
	flag.StringVar(&githubToken, "github-token", "", "")
	flag.StringVar(&githubOrg, "github-org", "kubernetes", "")
	flag.StringVar(&githubRepo, "github-repo", "kubernetes", "")
	flag.StringVar(&topLevelDir, "top-dir", "", "")
	flag.StringVar(&localRepo, "local-repo", "", "")
	flag.IntVar(&depth, "depth", 10, "")
	flag.Parse()

	now = time.Now()
}

func main() {
	if githubToken == "" {
		ExitError(errors.New("Github token isn't provided"))
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)

	// TODO: ignore list, e.g. "*generated*", "*proto*".
	fetchOwners(client, topLevelDir, 0)
}

func fetchOwners(client *github.Client, dir string, level int) {
	//log.Println("Collecting owners for path:", dir)
	if level > depth {
		return
	}

	_, directoryContent, _, err := client.Repositories.GetContents(githubOrg, githubRepo, dir, &github.RepositoryContentGetOptions{})
	if err != nil {
		ExitError(err)
	}
	fetchTopCommitters(client, dir, 3)
	for _, c := range directoryContent {
		if c.Type != nil && *c.Type == "dir" {
			fetchOwners(client, *c.Path, level+1)
		}
	}
}

func fetchTopCommitters(client *github.Client, dir string, limit int) {
	opt := &github.CommitsListOptions{
		Path: dir,
		//Since: now.AddDate(0, -6, 0),
		ListOptions: github.ListOptions{
			PerPage: 200,
		},
	}
	rank := map[string]commitWeight{}
	wt := 1.0
	for {
		commits, resp, err := client.Repositories.ListCommits(githubOrg, githubRepo, opt)
		if err != nil {
			ExitError(err)
		}

		for _, c := range commits {
			if c.Commit.Message == nil {
				//log.Printf("Commit.Message is nil, unexpected commit: %v\n", c.Commit.String())
				continue
			}
			if strings.HasPrefix(*c.Commit.Message, "Merge pull request") {
				continue
			}
			if c.Author == nil || c.Author.Login == nil {
				//log.Printf("Author or Author.Login is nil, unexpected commit: %v\n", c.Commit.String())
				continue
			}
			id := *c.Author.Login

			//rcommit, _, err := client.Repositories.GetCommit(*c.Author.Login, githubRepo, *c.SHA)
			//if err != nil {
			//	glog.Infof("Error fetching commit %v: %v", *c.SHA, err)
			//}
			//
			//addr := 0
			//if rcommit != nil && rcommit.Stats.Additions != nil {
			//	addr = *rcommit.Stats.Additions
			//}

			if val, ok := rank[id]; ok {
				rank[id] = commitWeight{Rank: val.Rank + (1 * wt), Lines: 0}
			} else {
				rank[id] = commitWeight{Rank: (1 * wt), Lines: 0}
			}
			wt += 0.001
			//fmt.Printf("CommitWeight: %+v, %+v\n", id, rank[id])

		}
		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}
	cr := committerRank{}
	for id, c := range rank {
		cr = append(cr, &committer{ID: id, CommitCount: c.Rank, LinesCount: c.Lines})
	}
	sort.Sort(cr)

	res := []string{}
	for i := 0; i < limit && i < len(cr); i++ {
		res = append(res, cr[i].ID)
	}

	fmt.Printf("path: %s, owners: %v\n", dir, res)
	if len(res) > 0 {
		writeOwnersFile(dir, res)
	}
}

// This function can be used to write a check-labels config file.
func writeOwnersFile(filePath string, assignees []string) error {
	fp, err := os.Create(path.Join(localRepo, filePath, "OWNERS"))
	if err != nil {
		return err
	}
	defer fp.Close()
	if err != nil {
		return err
	}
	m := map[string][]string{"assignees": assignees}
	res, _ := yaml.Marshal(m)
	io.WriteString(fp, fmt.Sprintf("%s\n", string(res)))
	return nil
}

type committer struct {
	ID          string
	CommitCount float64
	LinesCount  float64
}

type assignee struct {
}

type committerRank []*committer

func (s committerRank) Len() int { return len(s) }

func (s committerRank) Less(i, j int) bool {
	return (s[i].CommitCount > s[j].CommitCount) || (s[i].LinesCount > s[j].LinesCount)
}

func (s committerRank) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
