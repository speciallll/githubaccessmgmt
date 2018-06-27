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
	"strconv"
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

func main() {

	yamlTeams, yamlRepos := getDataFromYaml()
	githubTeams, githubRepos := getDataFromGithub()

	operations := TeamDiff(yamlTeams, githubTeams)
	operations = append(operations, RepoDiff(yamlRepos, githubRepos)...)

	// Need to add execute (and dry run)
}

func getDataFromYaml() (map[string]*TeamMap, map[string]*RepoMap) {

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
			t := TeamMap{}
			if _, ok := teams[team]; ok {
				t.Permission = "admin"
				teamsforRepo[team] = &t
			} else {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", team, repoName)
			}
		}
		for _, team := range repoValues.Write {
			t := TeamMap{}
			if _, ok := teams[team]; ok {
				t.Permission = "push"
				teamsforRepo[team] = &t
			} else {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", team, repoName)
			}
		}
		for _, team := range repoValues.Read {
			t := TeamMap{}
			if _, ok := teams[team]; ok {
				t.Permission = "pull"
				teamsforRepo[team] = &t
			} else {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", team, repoName)
			}
		}
		r.Teams = teamsforRepo
		reposMap[repoName] = &r
	}

	return teamsMap, reposMap

}

func getDataFromGithub() (map[string]*TeamMap, map[string]*RepoMap) {

	//get org parameter (defaults to splunk if not specified)
	orgPtr := flag.String("org", "splunk", "github organization")
	tokenPtr := flag.String("token", "", "github token")
	flag.Parse()

	//setup github client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *tokenPtr},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	//rate limits
	rateLimits, _, err := client.RateLimits(ctx)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Rate Limit:  %d\n", rateLimits.GetCore().Limit)
	fmt.Printf("Remaining Rate Limit:  %d\n", rateLimits.GetCore().Remaining)

	teams := make(map[string]*TeamMap)
	opts := &github.ListOptions{}
	for {
		githubTeams, resp, err := client.Organizations.ListTeams(ctx, *orgPtr, opts)
		if err != nil {
			fmt.Println(err)
		}

		for _, githubTeam := range githubTeams {
			t := TeamMap{}
			t.Id = githubTeam.GetID()
			usersMap := make(map[string]*User)
			u := User{}

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

		githubRepos, respForRepos, err := client.Repositories.ListByOrg(ctx, *orgPtr, optsForRepos)
		if err != nil {
			fmt.Println(err)
		}

		for _, githubRepo := range githubRepos {
			for {
				githubRepoTeams, resp, err := client.Repositories.ListTeams(ctx, *orgPtr, githubRepo.GetName(), opts)
				if err != nil {
					fmt.Println(err)
				}
				r := RepoMap{}
				teamsMap := make(map[string]*TeamMap)

				for _, githubRepoTeam := range githubRepoTeams {
					t := TeamMap{}
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

	return teams, repos

}

func TeamDiff(yamlTeams map[string]*TeamMap, githubTeams map[string]*TeamMap) (operations []string) {
	//var operations []string
	for yamlTeamName, yamlTeamValues := range yamlTeams {

		if githubTeam, ok := githubTeams[yamlTeamName]; ok {
			for yamlUser, _ := range yamlTeamValues.Users {
				if _, ok := githubTeam.Users[yamlUser]; !ok {
					operations = append(operations, "AddTeamMembership", strconv.FormatInt(githubTeam.Id, 10), yamlUser)
				}

			}
			for githubUser, _ := range githubTeam.Users {
				if _, ok := yamlTeamValues.Users[githubUser]; !ok {
					operations = append(operations, "RemoveTeamMembership", strconv.FormatInt(githubTeam.Id, 10), githubUser)
				}
			}
		} else {
			operations = append(operations, "CreateTeam", yamlTeamName)
			for yamlUser, _ := range yamlTeamValues.Users {
				operations = append(operations, "AddTeamMembership", strconv.Itoa(-1), yamlUser)
			}
		}
	}
	return operations
}

func RepoDiff(yamlRepos map[string]*RepoMap, githubRepos map[string]*RepoMap) (operations []string) {
	//var operations []string
	for yamlRepoName, yamlRepoValues := range yamlRepos {

		if githubRepo, ok := githubRepos[yamlRepoName]; ok {
			for yamlTeam, yamlTeamValues := range yamlRepoValues.Teams {
				if team, ok := githubRepo.Teams[yamlTeam]; ok {
					if team.Permission != yamlTeamValues.Permission {
						operations = append(operations, "AddTeamRepo", strconv.FormatInt(team.Id, 10), "orgPtr", yamlRepoName, yamlTeamValues.Permission)
					}
				} else {
					operations = append(operations, "AddTeamRepo", strconv.FormatInt(yamlTeamValues.Id, 10), "orgPtr", yamlRepoName, yamlTeamValues.Permission)
				}
			}

			for teamName, teamValues := range githubRepo.Teams {
				if _, ok := yamlRepoValues.Teams[teamName]; !ok {
					operations = append(operations, "RemoveTeamRepo", strconv.FormatInt(teamValues.Id, 10), "orgPtr", yamlRepoName)
				}
			}
		} else {
			fmt.Printf("ERROR:  Repo does not exist on Github for %s\n", yamlRepoName)
		}
	}
	return operations
}

/*

// create a new team
	newTeam := &github.NewTeam{
		Name: teamFromTeamYaml.Name,
	}

	newTeamCreated, _, err := client.Organizations.CreateTeam(ctx, *orgPtr, newTeam)
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	teamFromTeamYaml.Id = newTeamCreated.GetID()

// add team membership
	//     member - a normal member of the team
    //     maintainer - a team maintainer. Able to add/remove other team
    //                  members, promote other team members to team
    //                  maintainer, and edit the team’s name and description
    //
    // Default value is "member".
	_, _, err := client.Organizations.AddTeamMembership(ctx, teamFromTeamYaml.Id, userFromTeamYaml, nil)

	if err != nil {
		fmt.Printf("error: %v", err)
	}

// remove team membership

// add team to repo
	opts := &github.OrganizationAddTeamRepoOptions{}
	opts.Permission = "pull"

	_, err = client.Organizations.AddTeamRepo(ctx, githubRepoTeam.GetID(), *orgPtr, repoFromYaml.RepoName, opts)
	if err != nil {
		fmt.Println(err)
	}

// remove team from repo

// remove user from Org

*/
