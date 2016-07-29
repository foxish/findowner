package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"io/ioutil"
)

var (
	githubToken string
	githubOrg   string
	githubRepo  string
	topLevelDir string
	localRepo   string
	depth       int

	AuthList    map[string][]string
)

var now time.Time

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

	AuthList = map[string][]string {
		"ui": []string{"bryk"},
		"kubectl": []string{"bgrant0607"},
		"chinese": []string{"hurf"},
		"example": []string{"jeffmendoza"},
		"admission": []string{"erictune", "derekwaynecarr", "davidopp"},
		"api": []string{"bgrant0607"},
		"machinery": []string{"lavalamp"},
		"etcd": []string{"lavalamp"},
		"node": []string{"dchen1107"},
		"storage": []string{"saad-ali"},
		"network": []string{"thockin"},
		"sched": []string{"davidopp"},
		"control": []string{"bprashanth"},
		"replic": []string{"bprashanth"},
		"job": []string{"erictune", "soltysh"},
		"deploy": []string{"bgrant0607"},
		"nodecontroller": []string{"davidopp", "gmarek"},
		"pets": []string{"bprashanth"},
		"service": []string{"bprashanth"},
		"endpo": []string{"bprashanth"},
		"gc": []string{"mikedanese"},
		"garbage": []string{""},
		"namesp": []string{"derekwaynecarr"},
		"autosc": []string{"fgrzadkowski"},
		"quota": []string{"derekwaynecarr"},
		"account": []string{"liggitt"},
		"route": []string{"cjcullen"},
		"volu": []string{"jsafrane", "saad-ali"},
		"dns": []string{"ArtfulCoder"},
		"scala": []string{"wojtek-t"},
		"releas": []string{"david-mcmahon"},
		"ha": []string{"mikedanese"},
		"auth": []string{"erictune"},
		"security" : []string{"erictune"},
		"mesos": []string{"jdef"},
		"aws": []string{"justinsb"},
		"openstack": []string{"xsgordon", "idvoretskyi"},
	}

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
	fetchTopCommitters(client, dir, 3, false)
	for _, c := range directoryContent {
		if c.Type != nil {
			if *c.Type == "file" {
				if strings.Contains(*c.Name, "md") {
					//fmt.Println(*c.Name)
					fetchTopCommitters(client, *c.Path, 3, true)
				}
			} else if  *c.Type == "dir" {
				fetchOwners(client, *c.Path, level+1)
			}
		}
	}
}


func getRepoHistory(client *github.Client, dir string, repo string, rank *map[string]float64, wt float64) {
	rankP := *rank
	opt := &github.CommitsListOptions{
		Path:  dir,
		ListOptions: github.ListOptions{
			PerPage: 200,
		},
	}
	for {
		commits, resp, err := client.Repositories.ListCommits(githubOrg, repo, opt)
		fmt.Printf("%#v commits from %#v\n", len(commits), githubOrg + ":" + repo + ":" + dir)
		if err != nil {
			ExitError(err)
		}
		for _, c := range commits {
			if c.Commit.Message == nil {
				//log.Printf("Commit.Message is nil, unexpected commit: %v\n", c.Commit.String())
				continue
			}

			// no merges
			if strings.HasPrefix(*c.Commit.Message, "Merge pull request") {
				continue
			}

			// ignore gendocs
			if strings.Contains(*c.Commit.Message, "gendocs") {
				continue
			}

			// remove moving-to..
			if strings.Contains(*c.Commit.Message, "moving") {
				continue
			}

			if c.Author == nil || c.Author.Login == nil {
				//log.Printf("Author or Author.Login is nil, unexpected commit: %v\n", c.Commit.String())
				continue
			}
			id := *c.Author.Login
			rankP[id] = rankP[id] + wt
			wt += 0.001
		}
		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}
}

func fetchTopCommitters(client *github.Client, dir string, limit int, isFile bool) {
	rank := map[string]float64{}
	getRepoHistory(client, dir, githubRepo, &rank, 1)

	// if isFile, then Readme.md == Index.md
	dir2 := strings.Replace(dir, "index.md", "README.md", 1)
	getRepoHistory(client, dir2, "kubernetes", &rank, 1.5)

	//dir3 := path.Dir(dir)
	if !isFile {
		dir3 := dir + ".md"
		getRepoHistory(client, dir3, "kubernetes", &rank, 2)
	}





	cr := committerRank{}
	uniqs := map[string]bool{}
	for id, c := range rank {
		cr = append(cr, &committer{ID: id, CommitCount: c})
	}
	sort.Sort(cr)

	res := []string{}
	for i := 0; i < limit && i < len(cr); i++ {
		// remove from list.
		if cr[i].ID != "johndmulhausen" {
			res = append(res, cr[i].ID)
			uniqs[cr[i].ID] = true
		}
	}

	// go over all for current path:
	for k, v := range AuthList {
		if strings.Contains(dir, k) {
			for _, s := range v {
				if _, ok := uniqs[s]; !ok {
					res = append(res, s)
				}
			}
		}
	}
	sort.Strings(res)

	fmt.Printf("path: %s, owners: %v\n", dir, res)
	if len(res) > 0 {
		if isFile {
			// write to top section of MD.
			mdPath := path.Join(localRepo, dir)
			replaceTopSectionMd(mdPath, getYAML(res))
		} else {
			writeOwnersFile(dir, res)
		}
	}
}

func replaceTopSectionMd(filepath string, yamlString string) {
	read, _ := ioutil.ReadFile(filepath)
	newStr := fmt.Sprintf("---\n%s", yamlString)
	newContents := strings.Replace(string(read), "---", newStr, 1)
	ioutil.WriteFile(filepath, []byte(newContents), 0)
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
	io.WriteString(fp, fmt.Sprintf("%s\n", string(getYAML(assignees))))
	return nil
}

func getYAML(assignees []string) string {
	sort.Strings(assignees)
	m := map[string][]string{"assignees": assignees}
	res, _ := yaml.Marshal(m)
	return string(res)
}

type committer struct {
	ID          string
	CommitCount float64
}

type assignee struct {
}

type committerRank []*committer

func (s committerRank) Len() int { return len(s) }

func (s committerRank) Less(i, j int) bool {
	return s[i].CommitCount > s[j].CommitCount
}

func (s committerRank) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
