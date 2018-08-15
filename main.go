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

type UserYaml struct {
	ADUser string `yaml:"aduser"`
}

type TeamYaml struct {
	Id    int64    `yaml:"id"`
	Users []string `yaml:",flow"`
}

type RepoYaml struct {
	Admin []string `yaml:",flow"`
	Read  []string `yaml:",flow"`
	Write []string `yaml:",flow"`
}

type User struct {
	GitHubUserName 	   string
}

type Team struct {
	Name 	   string
	Id         int64
	Permission string
	Users      map[string]*User
}

type Repo struct {
	Name  string
	Teams map[string]*Team
}

type Operation interface {
	Execute(ctx context.Context, client *github.Client, org string, dryrun bool) error
}

type AddTeamMembershipOperation struct {
	team 	   *Team
	user       string
}

type RemoveTeamMembershipOperation struct {
	team 	   *Team
	user       string
}

type CreateTeamOperation struct {
	team       *Team
}

type UpdateTeamRepoPermissionOperation struct {
	team       *Team
	repoName   string
	permission string
}

type AddTeamRepoOperation struct {
	team       *Team
	repoName   string
	permission string
}

type RemoveTeamRepoOperation struct {
	team       *Team
	repoName   string
}

type RemoveOrgMemberOperation struct {
	userName string
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
	ops = append(ops, RepoDiff(yamlRepos, githubRepos, yamlTeams)...)

	for _, op := range ops {
		err := op.Execute(ctx, client, *orgPtr, *dryRunPtr)
		if err != nil {
			fmt.Println("ERROR in Execute(): ", err)
		}
	}
	if (*dryRunPtr){
		fmt.Println("INFO:  DRY RUN - NO UPDATES MADE")
	}
}

func getDataFromYaml() (map[string]*UserYaml, map[string]*Team, map[string]*Repo) {
	// get data from users.yaml and store in users
	users := make(map[string]*UserYaml)
	usersYamlFile, err := ioutil.ReadFile("users.yaml")
	if err != nil {
		fmt.Printf("usersYamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(usersYamlFile, &users)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// get data from teams.yaml and store in teamsYaml 
	teamsYaml := make(map[string]*TeamYaml)
	teamsYamlFile, err := ioutil.ReadFile("teams.yaml")
	if err != nil {
		fmt.Printf("teamsYamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(teamsYamlFile, &teamsYaml)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// check if users from teams.yaml exist in users.yaml
	// if not error and do not include in users to add to team
	teams := make(map[string]*Team)
	for teamName, teamAttributes := range teamsYaml {
		usersMap := make(map[string]*User)
		t := Team{}
		t.Name = teamName
		for _, user := range teamAttributes.Users {
			if _, ok := users[user]; !ok {
				fmt.Printf("ERROR: %s in teams.yaml for %s, but NOT in users.yaml\n", user, teamName)
			} else {
				u := User{} 
				u.GitHubUserName = user
				usersMap[user] = &u 
			}
		}
		t.Users = usersMap
		teams[teamName] = &t
	}

	// get data from repos.yaml and store in reposYaml
	reposYaml := make(map[string]*RepoYaml)
	reposYamlFile, err := ioutil.ReadFile("repos.yaml")
	if err != nil {
		fmt.Printf("reposYamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(reposYamlFile, &reposYaml)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// check if teams from repos.yaml exist in teams.yaml
	// if not error and do not include in teams to add to repos
	// teams can have repo permissions of admin, push or pull
	repos := make(map[string]*Repo)
	for repoName, repoAttributes := range reposYaml {
		teamsforRepo := make(map[string]*Team)
		r := Repo{}
		r.Name = repoName
		for _, team := range repoAttributes.Admin {
			if _, ok := teams[team]; ok {
				t := Team{}
				t.Name = team
				t.Permission = "admin"
				teamsforRepo[team] = &t
			} else {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", team, repoName)
			}
		}
		for _, team := range repoAttributes.Write {
			if _, ok := teams[team]; ok {
				t := Team{}
				t.Name = team
				t.Permission = "push"
				teamsforRepo[team] = &t
			} else {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", team, repoName)
			}
		}
		for _, team := range repoAttributes.Read {
			if _, ok := teams[team]; ok {
				t := Team{}
				t.Name = team
				t.Permission = "pull"
				teamsforRepo[team] = &t
			} else {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", team, repoName)
			}
		}
		r.Teams = teamsforRepo
		repos[repoName] = &r
	}
	return users, teams, repos
}

func getDataFromGithub(ctx context.Context, client *github.Client, org string) (map[string]*User, map[string]*Team, map[string]*Repo) {
	// get users for org from github and store in users
	users := make(map[string]*User)
	opt := &github.ListMembersOptions{}
	// loop to handle pagination from github
	for { 
		githubUsers, resp, err := client.Organizations.ListMembers(ctx, org, opt)
		if err != nil {
			fmt.Println(err)
		}
		for _, githubUser := range githubUsers {
			u := User{} 
			u.GitHubUserName = githubUser.GetLogin()
			users[githubUser.GetLogin()] = &u
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	// get teams for org from github and store in teams
	teams := make(map[string]*Team)
	opts := &github.ListOptions{}
	// loop to handle pagination from github ListTeams
	for {
		githubTeams, resp, err := client.Organizations.ListTeams(ctx, org, opts)
		if err != nil {
			fmt.Println(err)
		}
		// for every team on github, need to get users in team
		for _, githubTeam := range githubTeams {
			t := Team{}
			t.Id = githubTeam.GetID()
			t.Name = githubTeam.GetName()
			teamUsers := make(map[string]*User)
			optsForTeamMembers := &github.OrganizationListTeamMembersOptions{}
			optsForTeamMembers.ListOptions = github.ListOptions{}
			// loop to handle pagination from github ListTeamMembers
			for {
				githubUsers, respForTeamMembers, err := client.Organizations.ListTeamMembers(ctx, githubTeam.GetID(), optsForTeamMembers)
				if err != nil {
					fmt.Println(err)
				}
				// for every user on team from github, need to store in teams
				for _, githubUser := range githubUsers {
					u := User{} 
					u.GitHubUserName = githubUser.GetLogin()
					teamUsers[githubUser.GetLogin()] = &u
				}
				if respForTeamMembers.NextPage == 0 {
					break
				}
				optsForTeamMembers.ListOptions.Page = respForTeamMembers.NextPage
			}
			t.Users = teamUsers
			teams[githubTeam.GetName()] = &t
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// get repos for org from github and store in repos
	repos := make(map[string]*Repo)
	// loop to handle pagination from github ListByOrg
	for {
		optsForRepos := &github.RepositoryListByOrgOptions{}
		optsForRepos.ListOptions = github.ListOptions{}
		githubRepos, respForRepos, err := client.Repositories.ListByOrg(ctx, org, optsForRepos)
		if err != nil {
			fmt.Println(err)
		}
		// for every repo on github, need to get teams with permisions to repo
		for _, githubRepo := range githubRepos {
			// loop to handle pagination from github ListTeams
			for {
				githubRepoTeams, resp, err := client.Repositories.ListTeams(ctx, org, githubRepo.GetName(), opts)
				if err != nil {
					fmt.Println(err)
				}
				r := Repo{}
				repoTeams := make(map[string]*Team)
				// for every team on repo from github, need to store in repos with specific permissions granted
				for _, githubRepoTeam := range githubRepoTeams {
					if _, ok := teams[githubRepoTeam.GetName()]; ok {
						t := Team{}
						t.Id = githubRepoTeam.GetID()
						t.Name = githubRepoTeam.GetName()
						if githubRepoTeam.GetPermission() == "pull" {
							t.Permission = "pull"
						} else if githubRepoTeam.GetPermission() == "push" {
							t.Permission = "push"
						} else if githubRepoTeam.GetPermission() == "admin" {
							t.Permission = "admin"
						}
						repoTeams[githubRepoTeam.GetName()] = &t
					}
				}
				r.Teams = repoTeams
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

func UserDiff(yamlUsers map[string]*UserYaml, githubUsers map[string]*User) (ops []Operation) {
	// for all users in org on github, need to check if user also exists in yaml file
	for githubUser, _ := range githubUsers {
		// if github user does not exist in yaml file, need to remove from org
		if _, ok := yamlUsers[githubUser]; !ok {
			op := RemoveOrgMemberOperation{userName: githubUser}
			ops = append(ops, op)
		}
	}
	return ops
}

func TeamDiff(yamlTeams map[string]*Team, githubTeams map[string]*Team) (ops []Operation) {
	// for all teams in yaml file, need to check if team already exists on github
	for yamlTeamName, yamlTeamAttributes := range yamlTeams {
		// if team from yaml already exists on github, need to check if any users for team need to be updated on github
		if githubTeam, ok := githubTeams[yamlTeamName]; ok {
			yamlTeamAttributes.Id = githubTeam.Id
			// for all users in team from yaml file, need to check if user already is in team on github
			for yamlUser, _ := range yamlTeamAttributes.Users {
				// if user from yaml is not already on team on github, need to add user to team on github
				if _, ok := githubTeam.Users[yamlUser]; !ok {
					op := AddTeamMembershipOperation{team: yamlTeamAttributes, user: yamlUser}
					ops = append(ops, op)
				}
			}
			// for all users already on team on github, need to check if user is also in yaml
			for githubUser, _ := range githubTeam.Users {
				// if user from github is not also in yaml, need to remove team membership on github
				if _, ok := yamlTeamAttributes.Users[githubUser]; !ok {
					op := RemoveTeamMembershipOperation{team: githubTeam, user: githubUser}
					ops = append(ops, op)
				}
			}
		// if team in yaml does not exist on github, need to create team
		} else {  
			op := CreateTeamOperation{team: yamlTeamAttributes}
			ops = append(ops, op)
			// for users on team in yaml, need to add membership to newly created team
			for yamlUser, _ := range yamlTeamAttributes.Users {
				op := AddTeamMembershipOperation{team: yamlTeamAttributes, user: yamlUser}
				ops = append(ops, op)
			}
		}
	}
	return ops
}

func RepoDiff(yamlRepos map[string]*Repo, githubRepos map[string]*Repo, yamlTeams map[string]*Team) (ops []Operation) {
	// for all repos in yaml file, need to check if team already exists on github
	for yamlRepoName, yamlRepoAttributes := range yamlRepos {
		// if repo from yaml already exists on github, need to check if any teams for repo need to be updated on github
		if githubRepo, ok := githubRepos[yamlRepoName]; ok {
			// for all teams in repo from yaml file, need to check if team already is on repo on github
			for yamlTeam, yamlTeamAttributes := range yamlRepoAttributes.Teams {
				// if team from yaml is already on repo on github, need to check if permissions have changed in yaml
				if team, ok := githubRepo.Teams[yamlTeam]; ok {
					// if team from yaml has different permissions than team on github for repo, need to update permissions on github
					if team.Permission != yamlTeamAttributes.Permission {
						yamlTeamAttributes.Id = githubRepo.Teams[yamlTeam].Id
						op := UpdateTeamRepoPermissionOperation{team: yamlTeamAttributes, repoName: yamlRepoName, permission: yamlTeamAttributes.Permission }
						ops = append(ops, op)
					}
				// if team from yaml is not already on repo on github, need to add team to repo on github with appropriate permissions
				} else {
					op := AddTeamRepoOperation{team: yamlTeams[yamlTeam], repoName: yamlRepoName, permission: yamlTeamAttributes.Permission }
					ops = append(ops, op)
				}
			}
			// for all teams already on repo on github, need to check if team is also in yaml for repo
			for teamName, teamAttributes := range githubRepo.Teams {
				// if team from github is not also in yaml, need to remove team from repo on github
				if _, ok := yamlRepoAttributes.Teams[teamName]; !ok {
					op := RemoveTeamRepoOperation{team: teamAttributes, repoName: yamlRepoName}
					ops = append(ops, op)
				}
			}
		} else {
			fmt.Printf("ERROR: Repo does not exist on Github for %s\n", yamlRepoName)
		}
	}
	return ops
}

func (op AddTeamMembershipOperation) Execute(ctx context.Context, client *github.Client, org string, dryrun bool) error {
	rateLimits, _, err := client.RateLimits(ctx)
	fmt.Printf("Add user %s to team %s for org %s, Remaining Rate Limit %d\n", op.user, op.team.Name, org, rateLimits.GetCore().Remaining)
	if !dryrun {
		_, _, err = client.Organizations.AddTeamMembership(ctx, op.team.Id, op.user, nil)
	}
	return err
}

func (op RemoveTeamMembershipOperation) Execute(ctx context.Context, client *github.Client, org string, dryrun bool) error {
	rateLimits, _, err := client.RateLimits(ctx)
	fmt.Printf("Remove user %s from team %s for org %s, Remaining Rate Limit %d\n", op.user, op.team.Name, org, rateLimits.GetCore().Remaining)
	if !dryrun {
		if op.team.Id != 0{
			_, err = client.Organizations.RemoveTeamMembership(ctx, op.team.Id, op.user)
		} else{
			fmt.Printf("ERROR: Missing team ID to remove user %s from team %s for org %s, Remaining Rate Limit %d\n", op.user, op.team.Name, org, rateLimits.GetCore().Remaining)
		}
	}
	return err
}

func (op CreateTeamOperation) Execute(ctx context.Context, client *github.Client, org string, dryrun bool) error {
	rateLimits, _, err := client.RateLimits(ctx)
	fmt.Printf("Create new team %s for org %s, Remaining Rate Limit %d\n", op.team.Name, org, rateLimits.GetCore().Remaining)
	if !dryrun {
		// create a new team
		newTeam := &github.NewTeam{
			Name: op.team.Name,
		}
		var newGithubTeam *github.Team
		newGithubTeam, _, err = client.Organizations.CreateTeam(ctx, org, newTeam)
		op.team.Id = newGithubTeam.GetID()
	}
	return err
}

func (op UpdateTeamRepoPermissionOperation) Execute(ctx context.Context, client *github.Client, org string, dryrun bool) error {
	rateLimits, _, err := client.RateLimits(ctx)
	fmt.Printf("Update team %s to have permission %s for repo %s for org %s, Remaining Rate Limit %d\n", op.team.Name, op.permission, op.repoName, org, rateLimits.GetCore().Remaining)
	if !dryrun {
		// update team to repo permission
		if op.team.Id != 0{
			opts := &github.OrganizationAddTeamRepoOptions{}
			opts.Permission = op.permission
			_, err = client.Organizations.AddTeamRepo(ctx, op.team.Id, org, op.repoName, opts)
		}else{
			fmt.Printf("ERROR: Missing team ID to update team %s to have permission %s for repo %s for org %s, Remaining Rate Limit %d\n", op.team.Name, op.permission, op.repoName, org, rateLimits.GetCore().Remaining)
		}
	}
	return err
}

func (op AddTeamRepoOperation) Execute(ctx context.Context, client *github.Client, org string, dryrun bool) error {
	rateLimits, _, err := client.RateLimits(ctx)
	fmt.Printf("Add team %s to have permission %s for repo %s for org %s, Remaining Rate Limit %d\n", op.team.Name, op.permission, op.repoName, org, rateLimits.GetCore().Remaining)
	if !dryrun {
		if op.team.Id != 0{
			// add team to repo
			opts := &github.OrganizationAddTeamRepoOptions{}
			opts.Permission = op.permission
			_, err = client.Organizations.AddTeamRepo(ctx, op.team.Id, org, op.repoName, opts)
		}else{
			fmt.Printf("ERROR: Missing team ID to add team %s to have permission %s for repo %s for org %s, Remaining Rate Limit %d\n", op.team.Name, op.permission, op.repoName, org, rateLimits.GetCore().Remaining)
		}
	}
	return err
}

func (op RemoveTeamRepoOperation) Execute(ctx context.Context, client *github.Client, org string, dryrun bool) error {
	rateLimits, _, err := client.RateLimits(ctx)
	fmt.Printf("Remove team %s from repo %s for org %s, Remaining Rate Limit %d\n", op.team.Name, op.repoName, org, rateLimits.GetCore().Remaining)
	if !dryrun {
		if op.team.Id != 0{
			// remove team from repo
			_, err = client.Organizations.RemoveTeamRepo(ctx, op.team.Id, org, op.repoName)
		}else{
			fmt.Printf("ERROR: Missing team ID to remove team %s from repo %s for org %s, Remaining Rate Limit %d\n", op.team.Name, op.repoName, org, rateLimits.GetCore().Remaining)
		}
	}
	return err
}

func (op RemoveOrgMemberOperation) Execute(ctx context.Context, client *github.Client, org string, dryrun bool) error {
	rateLimits, _, err := client.RateLimits(ctx)
	fmt.Printf("Remove user %s from org %s, Remaining Rate Limit %d\n", op.userName, org, rateLimits.GetCore().Remaining)
	if !dryrun {
		// remove user from org
		_, err = client.Organizations.RemoveOrgMembership(ctx, op.userName, org)
	}
	return err
}
