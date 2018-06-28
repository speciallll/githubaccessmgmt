package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
)

type User struct {
	ADUser string `yaml:"aduser"`
}

type Team struct {
	Id    int64    `yaml:"id"`
	Users []string `yaml:",flow"`
}

type Repo struct {
	Admin []string `yaml:",flow"`
	Read  []string `yaml:",flow"`
	Write []string `yaml:",flow"`
}

type TeamMap struct {
	Id         int64
	Permission string
	Users      map[string]*User
}

type RepoMap struct {
	Teams map[string]*TeamMap
}

type Operation interface {
	Execute(ctx context.Context, client *github.Client, org string, dryrun bool)
}

type AddTeamMembershipOperation struct {
	teamId   int64
	teamName string
	user     string
}

type RemoveTeamMembershipOperation struct {
	teamId   int64
	teamName string
	user     string
}

type CreateTeamOperation struct {
	teamName string
	users    []AddTeamMembershipOperation
}

type UpdateTeamRepoPermissionOperation struct {
	teamId     int64
	teamName   string
	repoName   string
	permission string
}

type AddTeamRepoOperation struct {
	teamId     int64
	teamName   string
	repoName   string
	permission string
}

type RemoveTeamRepoOperation struct {
	teamId   int64
	teamName string
	repoName string
}

type RemoveOrgMemberOperation struct {
	userName string
}

func (op AddTeamMembershipOperation) Execute(ctx context.Context, client *github.Client, org string, dryrun bool) {
	rateLimits, _, err := client.RateLimits(ctx)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Add user %s to team %s for org %s, Remaining Rate Limit %d\n", op.user, op.teamName, org, rateLimits.GetCore().Remaining)
	if !dryrun {
		_, _, err := client.Organizations.AddTeamMembership(ctx, op.teamId, op.user, nil)
		if err != nil {
			fmt.Printf("error: %v", err)
		}
	}
}

func (op RemoveTeamMembershipOperation) Execute(ctx context.Context, client *github.Client, org string, dryrun bool) {
	rateLimits, _, err := client.RateLimits(ctx)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Remove user %s from team %s for org %s, Remaining Rate Limit %d\n", op.user, op.teamName, org, rateLimits.GetCore().Remaining)
	if !dryrun {
		_, err := client.Organizations.RemoveTeamMembership(ctx, op.teamId, op.user)
		if err != nil {
			fmt.Printf("error: %v", err)
		}
	}
}

func (op CreateTeamOperation) Execute(ctx context.Context, client *github.Client, org string, dryrun bool) {
	rateLimits, _, err := client.RateLimits(ctx)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Create new team %s for org %s, Remaining Rate Limit %d\n", op.teamName, org, rateLimits.GetCore().Remaining)
	if !dryrun {
		// create a new team
		newTeam := &github.NewTeam{
			Name: op.teamName,
		}
		newGithubTeam, _, err := client.Organizations.CreateTeam(ctx, org, newTeam)
		if err != nil {
			fmt.Printf("error: %v", err)
		}
		teamId := newGithubTeam.GetID()
		for _, u := range op.users {
			u.teamId = teamId
			u.Execute(ctx, client, org, dryrun)
		}
	}
}

func (op UpdateTeamRepoPermissionOperation) Execute(ctx context.Context, client *github.Client, org string, dryrun bool) {
	rateLimits, _, err := client.RateLimits(ctx)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Update team %s to have permission %s for repo %s for org %s, Remaining Rate Limit %d\n", op.teamName, op.permission, op.repoName, org, rateLimits.GetCore().Remaining)
	if !dryrun {
		// update team to repo permission
		opts := &github.OrganizationAddTeamRepoOptions{}
		opts.Permission = op.permission

		_, err := client.Organizations.AddTeamRepo(ctx, op.teamId, org, op.repoName, opts)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func (op AddTeamRepoOperation) Execute(ctx context.Context, client *github.Client, org string, dryrun bool) {
	rateLimits, _, err := client.RateLimits(ctx)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Add team %s to have permission %s for repo %s for org %s, Remaining Rate Limit %d\n", op.teamName, op.permission, op.repoName, org, rateLimits.GetCore().Remaining)
	if !dryrun {
		// add team to repo
		opts := &github.OrganizationAddTeamRepoOptions{}
		opts.Permission = op.permission

		_, err := client.Organizations.AddTeamRepo(ctx, op.teamId, org, op.repoName, opts)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func (op RemoveTeamRepoOperation) Execute(ctx context.Context, client *github.Client, org string, dryrun bool) {
	rateLimits, _, err := client.RateLimits(ctx)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Remove team %s from repo %s for org %s, Remaining Rate Limit %d\n", op.teamName, op.repoName, org, rateLimits.GetCore().Remaining)
	if !dryrun {
		// remove team from repo
		_, err := client.Organizations.RemoveTeamRepo(ctx, op.teamId, org, op.repoName)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func (op RemoveOrgMemberOperation) Execute(ctx context.Context, client *github.Client, org string, dryrun bool) {
	rateLimits, _, err := client.RateLimits(ctx)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Remove user %s from org %s, Remaining Rate Limit %d\n", op.userName, org, rateLimits.GetCore().Remaining)
	if !dryrun {
		// remove team from repo
		_, err := client.Organizations.RemoveOrgMembership(ctx, op.userName, org)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func main() {

	//params
	orgPtr := flag.String("org", "splunk", "github organization")
	tokenPtr := flag.String("token", "", "github token")
	dryRunPtr := flag.Bool("dryrun", true, "if dryrun true, then do not update github")
	flag.Parse()

	//setup github client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *tokenPtr},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	yamlUsers, yamlTeams, yamlRepos := getDataFromYaml()
	githubUsers, githubTeams, githubRepos := getDataFromGithub(ctx, client, *orgPtr)

	ops := UserDiff(yamlUsers, githubUsers)
	ops = append(ops, TeamDiff(yamlTeams, githubTeams)...)
	ops = append(ops, RepoDiff(yamlRepos, githubRepos)...)

	for _, op := range ops {
		op.Execute(ctx, client, *orgPtr, *dryRunPtr)
	}
}

func getDataFromYaml() (map[string]*User, map[string]*TeamMap, map[string]*RepoMap) {

	// get data from users.yaml
	users := make(map[string]*User)
	usersYamlFile, err := ioutil.ReadFile("users.yaml")
	if err != nil {
		fmt.Printf("usersYamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(usersYamlFile, &users)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// get data from teams.yaml
	teams := make(map[string]*Team)
	teamsYamlFile, err := ioutil.ReadFile("teams.yaml")
	if err != nil {
		fmt.Printf("teamsYamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(teamsYamlFile, &teams)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// check if users from teams.yaml exist in users.yaml
	teamsMap := make(map[string]*TeamMap)
	for teamName, teamValues := range teams {
		usersMap := make(map[string]*User)
		t := TeamMap{}
		u := User{} // should be below in for loop if ever need to store something for user
		for _, user := range teamValues.Users {
			if _, ok := users[user]; !ok {
				fmt.Printf("ERROR: %s in teams.yaml for %s, but NOT in users.yaml\n", user, teamName)
			} else {
				usersMap[user] = &u //temp holding place until something needs to be stored for user
			}
		}
		t.Users = usersMap
		teamsMap[teamName] = &t
	}

	// get data from repos.yaml
	repos := make(map[string]*Repo)
	reposYamlFile, err := ioutil.ReadFile("repos.yaml")
	if err != nil {
		fmt.Printf("reposYamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(reposYamlFile, &repos)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// check if teams from repos.yaml exist in teams.yaml
	reposMap := make(map[string]*RepoMap)
	for repoName, repoValues := range repos {
		teamsforRepo := make(map[string]*TeamMap)
		r := RepoMap{}
		for _, team := range repoValues.Admin {
			//t := TeamMap{}
			if t, ok := teamsMap[team]; ok {
				t.Permission = "admin"
				teamsforRepo[team] = t
			} else {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", team, repoName)
			}
		}
		for _, team := range repoValues.Write {
			//t := TeamMap{}
			if t, ok := teamsMap[team]; ok {
				t.Permission = "push"
				teamsforRepo[team] = t
			} else {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", team, repoName)
			}
		}
		for _, team := range repoValues.Read {
			//t := TeamMap{}
			if t, ok := teamsMap[team]; ok {
				t.Permission = "pull"
				teamsforRepo[team] = t
			} else {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", team, repoName)
			}
		}
		r.Teams = teamsforRepo
		reposMap[repoName] = &r
	}

	return users, teamsMap, reposMap

}

func getDataFromGithub(ctx context.Context, client *github.Client, org string) (map[string]*User, map[string]*TeamMap, map[string]*RepoMap) {

	users := make(map[string]*User)
	opt := &github.ListMembersOptions{}
	for {
		githubUsers, resp, err := client.Organizations.ListMembers(ctx, org, opt)
		if err != nil {
			fmt.Println(err)
		}
		u := User{} // should be below in for loop if ever need to store something for user
		for _, githubUser := range githubUsers {
			users[githubUser.GetLogin()] = &u
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	teams := make(map[string]*TeamMap)
	opts := &github.ListOptions{}
	for {

		githubTeams, resp, err := client.Organizations.ListTeams(ctx, org, opts)
		if err != nil {
			fmt.Println(err)
		}

		for _, githubTeam := range githubTeams {
			t := TeamMap{}
			t.Id = githubTeam.GetID()
			usersMap := make(map[string]*User)
			u := User{} // should be below in for loop if ever need to store something for user

			optsForTeamMembers := &github.OrganizationListTeamMembersOptions{}
			optsForTeamMembers.ListOptions = github.ListOptions{}

			for {
				githubUsers, respForTeamMembers, err := client.Organizations.ListTeamMembers(ctx, githubTeam.GetID(), optsForTeamMembers)
				if err != nil {
					fmt.Println(err)
				}

				for _, githubUser := range githubUsers {
					usersMap[githubUser.GetLogin()] = &u
				}
				if respForTeamMembers.NextPage == 0 {
					break
				}
				optsForTeamMembers.ListOptions.Page = respForTeamMembers.NextPage
			}
			t.Users = usersMap
			teams[githubTeam.GetName()] = &t

		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	repos := make(map[string]*RepoMap)
	for {
		optsForRepos := &github.RepositoryListByOrgOptions{}
		optsForRepos.ListOptions = github.ListOptions{}

		githubRepos, respForRepos, err := client.Repositories.ListByOrg(ctx, org, optsForRepos)
		if err != nil {
			fmt.Println(err)
		}

		for _, githubRepo := range githubRepos {
			for {
				githubRepoTeams, resp, err := client.Repositories.ListTeams(ctx, org, githubRepo.GetName(), opts)
				if err != nil {
					fmt.Println(err)
				}
				r := RepoMap{}
				teamsMap := make(map[string]*TeamMap)

				for _, githubRepoTeam := range githubRepoTeams {
					t := TeamMap{}
					t.Id = githubRepoTeam.GetID()
					if githubRepoTeam.GetPermission() == "pull" {
						t.Permission = "pull"
					} else if githubRepoTeam.GetPermission() == "push" {
						t.Permission = "push"
					} else if githubRepoTeam.GetPermission() == "admin" {
						t.Permission = "admin"
					}
					teamsMap[githubRepoTeam.GetName()] = &t
				}

				r.Teams = teamsMap
				repos[githubRepo.GetName()] = &r

				if resp.NextPage == 0 {
					break
				}
				opts.Page = resp.NextPage
			}
		}

		if respForRepos.NextPage == 0 {
			break
		}
		optsForRepos.Page = respForRepos.NextPage
	}

	return users, teams, repos

}

func UserDiff(yamlUsers map[string]*User, githubUsers map[string]*User) (ops []Operation) {
	for githubUser, _ := range githubUsers {
		if _, ok := yamlUsers[githubUser]; !ok {
			op := RemoveOrgMemberOperation{userName: githubUser}
			ops = append(ops, op)
		}
	}

	return ops
}

func TeamDiff(yamlTeams map[string]*TeamMap, githubTeams map[string]*TeamMap) (ops []Operation) {
	for yamlTeamName, yamlTeamValues := range yamlTeams {

		if githubTeam, ok := githubTeams[yamlTeamName]; ok {
			for yamlUser, _ := range yamlTeamValues.Users {
				if _, ok := githubTeam.Users[yamlUser]; !ok {
					op := AddTeamMembershipOperation{teamId: githubTeam.Id, teamName: yamlTeamName, user: yamlUser}
					ops = append(ops, op)
				}

			}
			for githubUser, _ := range githubTeam.Users {
				if _, ok := yamlTeamValues.Users[githubUser]; !ok {
					op := RemoveTeamMembershipOperation{teamId: githubTeam.Id, teamName: yamlTeamName, user: githubUser}
					ops = append(ops, op)
				}
			}
		} else {
			op := CreateTeamOperation{teamName: yamlTeamName}
			var childOps []AddTeamMembershipOperation
			for yamlUser, _ := range yamlTeamValues.Users {
				op := AddTeamMembershipOperation{teamName: yamlTeamName, user: yamlUser}
				childOps = append(childOps, op)
			}
			op.users = childOps
			ops = append(ops, op)
		}
	}
	return ops
}

func RepoDiff(yamlRepos map[string]*RepoMap, githubRepos map[string]*RepoMap) (ops []Operation) {
	for yamlRepoName, yamlRepoValues := range yamlRepos {

		if githubRepo, ok := githubRepos[yamlRepoName]; ok {
			for yamlTeam, yamlTeamValues := range yamlRepoValues.Teams {
				if team, ok := githubRepo.Teams[yamlTeam]; ok {
					if team.Permission != yamlTeamValues.Permission {
						op := UpdateTeamRepoPermissionOperation{teamId: team.Id, teamName: yamlTeam, repoName: yamlRepoName, permission: yamlTeamValues.Permission}
						ops = append(ops, op)
					}
				} else {
					op := AddTeamRepoOperation{teamId: yamlTeamValues.Id, teamName: yamlTeam, repoName: yamlRepoName, permission: yamlTeamValues.Permission}
					ops = append(ops, op)
				}
			}

			for teamName, teamValues := range githubRepo.Teams {
				if _, ok := yamlRepoValues.Teams[teamName]; !ok {
					op := RemoveTeamRepoOperation{teamId: teamValues.Id, teamName: teamName, repoName: yamlRepoName}
					ops = append(ops, op)
				}
			}
		} else {
			fmt.Printf("ERROR: Repo does not exist on Github for %s\n", yamlRepoName)
		}
	}
	return ops
}
