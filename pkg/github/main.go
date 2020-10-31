package github

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	git "github.com/olblak/updateCli/pkg/git/generic"
	"github.com/olblak/updateCli/pkg/tmp"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

// Github contains settings to interact with Github
type Github struct {
	Owner                  string
	Description            string
	PullRequestDescription Changelog `yaml:"-"`
	Repository             string
	Username               string
	Token                  string
	URL                    string
	Version                string
	Name                   string
	Directory              string
	Branch                 string
	remoteBranch           string
	User                   string
	Email                  string
}

// GetDirectory returns the local git repository path.
func (g *Github) GetDirectory() (directory string) {
	return g.Directory
}

// Check verifies if mandatory Github parameters are provided and return false if not.
func (g *Github) Check() (bool, error) {
	ok := true
	required := []string{}

	if g.Token == "" {
		required = append(required, "token")
	}

	if g.Username == "" {
		required = append(required, "username")
	}

	if g.Owner == "" {
		required = append(required, "owner")
	}

	if g.Repository == "" {
		required = append(required, "repository")
	}

	if len(required) > 0 {
		err := fmt.Errorf("\u2717 Github parameter(s) required: [%v]", strings.Join(required, ","))
		return false, err
	}

	return ok, nil

}

// Init set default Github parameters if not set.
func (g *Github) Init(source string, name string) error {
	g.Version = source
	g.Name = name
	g.remoteBranch = git.SanitizeBranchName(fmt.Sprintf("updatecli/%v/%v", g.Name, g.Version))
	g.setDirectory()

	if ok, err := g.Check(); !ok {
		return err
	}
	return nil
}

func (g *Github) setDirectory() {

	if g.Directory == "" {
		g.Directory = path.Join(tmp.Directory, g.Owner, g.Repository)
	}

	if _, err := os.Stat(g.Directory); os.IsNotExist(err) {

		err := os.MkdirAll(g.Directory, 0755)
		if err != nil {
			fmt.Println(err)
		}
	}
}

// Clean deletes github working directory.
func (g *Github) Clean() {
	os.RemoveAll(g.Directory)
}

// Clone run `git clone`.
func (g *Github) Clone() string {

	URL := fmt.Sprintf("https://github.com/%v/%v.git",
		g.Owner,
		g.Repository)

	g.setDirectory()

	err := git.Clone(g.Username, g.Token, URL, g.GetDirectory())

	if err != nil {
		fmt.Println(err)
	}

	err = git.Checkout(g.Branch, g.remoteBranch, g.GetDirectory())

	if err != nil {
		fmt.Println(err)
	}

	return g.Directory
}

// Commit run `git commit`.
func (g *Github) Commit(message string) {
	err := git.Commit(g.User, g.Email, message, g.GetDirectory())
	if err != nil {
		fmt.Println(err)
	}
}

// Checkout create and then uses a temporary git branch.
func (g *Github) Checkout() {
	err := git.Checkout(g.Branch, g.remoteBranch, g.Directory)
	if err != nil {
		fmt.Println(err)
	}
}

// Add run `git add`.
func (g *Github) Add(files []string) {

	err := git.Add(files, g.Directory)
	if err != nil {
		fmt.Println(err)
	}
}

// Push run `git push` then open a pull request on Github if not already created.
func (g *Github) Push() {

	err := git.Push(g.Username, g.Token, g.GetDirectory())
	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("\n")

	g.OpenPullRequest()
}

// OpenPullRequest creates a new pull request.
func (g *Github) OpenPullRequest() {

	/*
		mutation($input: CreatePullRequestInput!){
			createPullRequest(input:$input){
				pullRequest{
					url
				}
			}
		}
		{

		"input":{
			"baseRefName": "x" ,
			"repositoryId":"y",
			"headRefName": "z",
			"title",
			"body",
		}


	*/
	client := g.NewClient()

	var mutation struct {
		createPullRequest struct {
			pullRequest PullRequest
		} `graphql:"createPullRequest(input: $input)"`
	}

	fmt.Println("Opening Github pull request")

	title := fmt.Sprintf("[updatecli] Update %v version to %v", g.Name, g.Version)
	repositoryID, err := g.queryRepositoryID()
	maintainerCanModify := true
	draft := false

	bodyPR, err := SetBody(g.PullRequestDescription)

	if err != nil {
		fmt.Println(err)
		return
	}

	if ok, url, err := g.isPRExist(); ok && err == nil {
		fmt.Printf("Pull Request titled '%v' already exist at\n\t%s\n", title, url)
		return
	}

	if err != nil {
		fmt.Println(err)
	}

	input := githubv4.CreatePullRequestInput{
		BaseRefName:         githubv4.String(g.Branch),
		RepositoryID:        githubv4.String(repositoryID),
		HeadRefName:         githubv4.String(g.remoteBranch),
		Title:               githubv4.String(title),
		Body:                githubv4.NewString(githubv4.String(bodyPR)),
		MaintainerCanModify: githubv4.NewBoolean(githubv4.Boolean(maintainerCanModify)),
		Draft:               githubv4.NewBoolean(githubv4.Boolean(draft)),
	}

	err = client.Mutate(context.Background(), &mutation, input, nil)

	return

}

//// isPRExist checks if an open pull request already exist and is in an open state.
func (g *Github) isPRExist() (bool, string, error) {
	/*
			https://developer.github.com/v4/explorer/
		# Query
		query getPullRequests(
			$owner: String!,
			$name:String!,
			$baseRefName:String!,
			$headRefName:String!){
				repository(owner: $owner, name: $name) {
					pullRequests(baseRefName: $baseRefName, headRefName: $headRefName, last: 1) {
						nodes {
							state
							id
						}
					}
				}
			}
		}
		# Variables
		{
			"owner": "olblak",
			"name": "charts",
			"baseRefName": "master",
			"headRefName": "updatecli/HelmChart/2.4.0"
		}
	*/

	client := g.NewClient()

	var query struct {
		Repository struct {
			PullRequests struct {
				Nodes []PullRequest
			} `graphql:"pullRequests(baseRefName: $baseRefName, headRefName: $headRefName, last: 1, states: OPEN)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner":       githubv4.String(g.Owner),
		"name":        githubv4.String(g.Repository),
		"baseRefName": githubv4.String(g.Branch),
		"headRefName": githubv4.String(g.remoteBranch),
	}

	err := client.Query(context.Background(), &query, variables)

	if err != nil {
		fmt.Println(err)
		return false, "", err
	}

	if len(query.Repository.PullRequests.Nodes) > 0 {
		return true, query.Repository.PullRequests.Nodes[0].Url, nil
	}

	return false, "", nil
}

//NewClient return a new client
func (g *Github) NewClient() *githubv4.Client {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: g.Token},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	if g.URL == "" || strings.HasSuffix(g.URL, "github.com") {
		return githubv4.NewClient(httpClient)

	}

	return githubv4.NewEnterpriseClient(os.Getenv(g.Token), httpClient)
}

func (g *Github) queryRepositoryID() (string, error) {
	/*
		query($owner: String!, $name: String!) {
			repository(owner: $owner, name: $name){
				id
			}
		}
	*/

	client := g.NewClient()

	var query struct {
		Repository struct {
			ID string
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner": githubv4.String(g.Owner),
		"name":  githubv4.String(g.Repository),
	}

	err := client.Query(context.Background(), &query, variables)

	if err != nil {
		fmt.Println(err)
		return "", err
	}

	return query.Repository.ID, nil

}
