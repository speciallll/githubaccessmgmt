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
	Id    int64
	Users map[string]*User
}

type RepoMap struct {
	Admin map[string]*TeamMap
	Read  map[string]*TeamMap
	Write map[string]*TeamMap
}

func main() {

	yamlTeams, yamlRepos := getDataFromYaml()
	githubTeams, githubRepos := getDataFromGithub()

	TeamDiff(yamlTeams, githubTeams)
	RepoDiff(yamlRepos, githubRepos)

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
		u := User{}
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
		teamsAdminMap := make(map[string]*TeamMap)
		teamsReadMap := make(map[string]*TeamMap)
		teamsWriteMap := make(map[string]*TeamMap)
		r := RepoMap{}
		t := TeamMap{}
		for _, adminTeam := range repoValues.Admin {
			if _, ok := teams[adminTeam]; !ok {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", adminTeam, repoName)
			} else {
				teamsAdminMap[adminTeam] = &t //temp holding place until something needs to be stored for team
			}
		}
		for _, readTeam := range repoValues.Read {
			if _, ok := teams[readTeam]; !ok {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", readTeam, repoName)
			} else {
				teamsReadMap[readTeam] = &t //temp holding place until something needs to be stored for team
			}
		}
		for _, writeTeam := range repoValues.Write {
			if _, ok := teams[writeTeam]; !ok {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", writeTeam, repoName)
			} else {
				teamsWriteMap[writeTeam] = &t //temp holding place until something needs to be stored for team
			}
		}
		r.Admin = teamsAdminMap
		r.Read = teamsReadMap
		r.Write = teamsWriteMap

		reposMap[repoName] = &r
	}

	return teamsMap, reposMap

}

func getDataFromGithub() (map[string]*TeamMap, map[string]*RepoMap) {

	//get org parameter (defaults to splunk if not specified)
	orgPtr := flag.String("org", "splunk", "github organization")
	flag.Parse()
	fmt.Printf("org: %s\n", *orgPtr)

	//setup github client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: "..."},
	) //personal access token for now
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
				t := TeamMap{}
				teamsAdminMap := make(map[string]*TeamMap)
				teamsReadMap := make(map[string]*TeamMap)
				teamsWriteMap := make(map[string]*TeamMap)

				for _, githubRepoTeam := range githubRepoTeams {
					if githubRepoTeam.GetPermission() == "pull" {
						teamsReadMap[githubRepoTeam.GetName()] = &t
					} else if githubRepoTeam.GetPermission() == "push" {
						teamsWriteMap[githubRepoTeam.GetName()] = &t
					} else if githubRepoTeam.GetPermission() == "admin" {
						teamsAdminMap[githubRepoTeam.GetName()] = &t
					}
				}

				r.Admin = teamsAdminMap
				r.Read = teamsReadMap
				r.Write = teamsWriteMap

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

func TeamDiff(yamlTeams map[string]*TeamMap, githubTeams map[string]*TeamMap) {
	for yamlTeamName, yamlTeamValues := range yamlTeams {

		if githubTeam, ok := githubTeams[yamlTeamName]; ok {
			fmt.Printf("Found Match on Github for Team %s\n", yamlTeamName)
			for yamlUser, _ := range yamlTeamValues.Users {
				if _, ok := githubTeam.Users[yamlUser]; ok {
					fmt.Printf("Found Match on Github user %s for Team %s\n", yamlUser, yamlTeamName)
				} else {
					fmt.Printf("Need to add member %s to team  %s on Github\n", yamlUser, yamlTeamName)
					// TODO make map AddTeamMembership, githubTeams[yamlTeamName].Id, yamlUser
				}

			}
			for githubUser, _ := range githubTeam.Users {
				if _, ok := yamlTeamValues.Users[githubUser]; !ok {
					fmt.Printf("Need to delete %s from team  %s on Github\n", githubUser, yamlTeamName)
				}
			}
		} else {
			fmt.Printf("Need to create team on Github %s\n", yamlTeamName)
			// TODO make map CreateTeam, yamlTeamName with children
			for yamlUser, _ := range yamlTeamValues.Users {
				fmt.Printf("Need to add member %s to team  %s on Github\n", yamlUser, yamlTeamName)
				// TODO make submap AddTeamMembership, githubTeams[yamlTeamName].Id, yamlUser
			}
		}

	}
}

func RepoDiff(yamlRepos map[string]*RepoMap, githubRepos map[string]*RepoMap) {
	for yamlRepoName, yamlRepoValues := range yamlRepos {

		if githubRepo, ok := githubRepos[yamlRepoName]; ok {
			fmt.Printf("Found Match on Github for Repo %s\n", yamlRepoName)

			for yamlReadTeam, _ := range yamlRepoValues.Read {
				if _, ok := githubRepo.Read[yamlReadTeam]; ok {
					fmt.Printf("Found Match on Github %s for Team %s as READ\n", yamlReadTeam, yamlRepoName)
				} else {
					fmt.Printf("Need to add team %s as READ to repo  %s on Github\n", yamlReadTeam, yamlRepoName)
				}

			}

			for yamlWriteTeam, _ := range yamlRepoValues.Write {
				if _, ok := githubRepo.Write[yamlWriteTeam]; ok {
					fmt.Printf("Found Match on Github %s for Team %s as WRITE\n", yamlWriteTeam, yamlRepoName)
				} else {
					fmt.Printf("Need to add team %s as WRITE to repo  %s on Github\n", yamlWriteTeam, yamlRepoName)
				}

			}

			for yamlAdminTeam, _ := range yamlRepoValues.Admin {
				if _, ok := githubRepo.Admin[yamlAdminTeam]; ok {
					fmt.Printf("Found Match on Github %s for Team %s as ADMIN\n", yamlAdminTeam, yamlRepoName)
				} else {
					fmt.Printf("Need to add team %s as ADMIN to repo  %s on Github\n", yamlAdminTeam, yamlRepoName)
				}

			}

			teams := make(map[string]*TeamMap)
			for key, value := range githubRepo.Read {
				teams[key] = value
			}
			for key, value := range githubRepo.Write {
				teams[key] = value
			}
			for key, value := range githubRepo.Admin {
				teams[key] = value
			}

			for githubTeam, _ := range teams {
				teamFoundinYaml := false
				if _, ok := yamlRepoValues.Admin[githubTeam]; ok {
					teamFoundinYaml = true
				}
				if _, ok := yamlRepoValues.Read[githubTeam]; ok {
					teamFoundinYaml = true
				}
				if _, ok := yamlRepoValues.Write[githubTeam]; ok {
					teamFoundinYaml = true
				}
				if !teamFoundinYaml {
					fmt.Printf("Need to delete team %s from repo %s on Github\n", githubTeam, yamlRepoName)
				}
			}
		} else {
			fmt.Printf("ERROR:  Repo does not exist on Github for %s\n", yamlRepoName)

		}

	}
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
