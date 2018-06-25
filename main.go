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

// TODO STILL NEED TO HANDLE
// if teams are removed from repos

func main() {

	usersFromUserYaml, teamsFromTeamYaml, reposFromRepoYaml := getDataFromYaml()
	teamsUsersFromGithub, reposTeamsFromGithub := getDataFromGithub()

	TeamUserDiff(teamsFromTeamYaml, usersFromUserYaml, teamsUsersFromGithub)
	RepoTeamDiff(reposFromRepoYaml, teamsFromTeamYaml, reposTeamsFromGithub)

	// Need to add execute (and dry run)
}

func getDataFromYaml() (map[string]*User, map[string]*Team, map[string]*Repo) {

	// get data from users.yaml
	usersFromUserYaml := make(map[string]*User)
	usersYamlFile, err := ioutil.ReadFile("users.yaml")
	if err != nil {
		fmt.Printf("usersYamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(usersYamlFile, &usersFromUserYaml)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	// TODO maybe add to default splunk team that gets read access to all repos?

	// get data from teams.yaml
	teamsFromTeamYaml := make(map[string]*Team)

	teamsYamlFile, err := ioutil.ReadFile("teams.yaml")

	if err != nil {
		fmt.Printf("teamsYamlFile.Get err   #%v ", err)
	}

	err = yaml.Unmarshal(teamsYamlFile, &teamsFromTeamYaml)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// get data from repos.yaml
	reposFromRepoYaml := make(map[string]*Repo)

	reposYamlFile, err := ioutil.ReadFile("repos.yaml")

	if err != nil {
		fmt.Printf("reposYamlFile.Get err   #%v ", err)
	}

	err = yaml.Unmarshal(reposYamlFile, &reposFromRepoYaml)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	return usersFromUserYaml, teamsFromTeamYaml, reposFromRepoYaml

}

func getDataFromGithub() (map[string]*Team, map[string]*Repo) {

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

	teamsUsersFromGithub := make(map[string]*Team)
	opts := &github.ListOptions{}
	for {
		githubTeams, resp, err := client.Organizations.ListTeams(ctx, *orgPtr, opts)
		if err != nil {
			fmt.Println(err)
		}

		for _, githubTeam := range githubTeams {
			t := Team{}
			t.Id = githubTeam.GetID()
			var users []string

			optsForTeamMembers := &github.OrganizationListTeamMembersOptions{}
			optsForTeamMembers.ListOptions = github.ListOptions{}

			for {
				githubUsers, respForTeamMembers, err := client.Organizations.ListTeamMembers(ctx, githubTeam.GetID(), optsForTeamMembers)
				if err != nil {
					fmt.Println(err)
				}

				for _, githubUser := range githubUsers {
					users = append(users, githubUser.GetLogin())
				}
				if respForTeamMembers.NextPage == 0 {
					break
				}
				optsForTeamMembers.ListOptions.Page = respForTeamMembers.NextPage
			}
			t.Users = users
			teamsUsersFromGithub[githubTeam.GetName()] = &t

		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	reposTeamsFromGithub := make(map[string]*Repo)
	for {
		optsForRepos := &github.RepositoryListByOrgOptions{}
		optsForRepos.ListOptions = github.ListOptions{}

		githubRepos, respForRepos, err := client.Repositories.ListByOrg(ctx, *orgPtr, optsForRepos)
		if err != nil {
			fmt.Println(err)
		}

		for _, githubRepo := range githubRepos {
			r := Repo{}

			for {
				githubRepoTeams, resp, err := client.Repositories.ListTeams(ctx, *orgPtr, githubRepo.GetName(), opts)
				if err != nil {
					fmt.Println(err)
				}

				var admin []string
				var read []string
				var write []string

				for _, githubRepoTeam := range githubRepoTeams {
					if githubRepoTeam.GetPermission() == "pull" {
						read = append(read, githubRepoTeam.GetName())
					} else if githubRepoTeam.GetPermission() == "push" {
						write = append(write, githubRepoTeam.GetName())
					} else if githubRepoTeam.GetPermission() == "admin" {
						admin = append(admin, githubRepoTeam.GetName())
					}

				}

				r.Admin = admin
				r.Read = read
				r.Write = write

				reposTeamsFromGithub[githubRepo.GetName()] = &r

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

	return teamsUsersFromGithub, reposTeamsFromGithub

}

func TeamUserDiff(teamsFromTeamYaml map[string]*Team, usersFromUserYaml map[string]*User, teamsUsersFromGithub map[string]*Team) {
	for teamFromTeamYaml, usersFromTeamYaml := range teamsFromTeamYaml {

		if team, ok := teamsUsersFromGithub[teamFromTeamYaml]; ok {
			fmt.Printf("Found Match on Github for Team %s\n", teamFromTeamYaml)
			for _, userFromTeamYaml := range usersFromTeamYaml.Users {
				if _, ok := usersFromUserYaml[userFromTeamYaml]; !ok {
					fmt.Printf("ERROR: %s in teams.yaml for %s, but NOT in users.yaml\n", userFromTeamYaml, teamFromTeamYaml)
				} else {
					for _, user := range team.Users {
						if user == userFromTeamYaml {
							fmt.Printf("Found Match on Github user %s for Team %s\n", userFromTeamYaml, teamFromTeamYaml)
						} else {
							fmt.Printf("Need to add member %s to team  %s on Github\n", userFromTeamYaml, teamFromTeamYaml)
						}
					}
					// TODO STILL NEED TO HANDLE if users are removed from existing teams
				}
			}
		} else {
			fmt.Printf("Need to create team on Github %s\n", teamFromTeamYaml)
			for _, userFromTeamYaml := range usersFromTeamYaml.Users {
				if _, ok := usersFromUserYaml[userFromTeamYaml]; ok {
					fmt.Printf("Need to add member %s to team  %s on Github\n", userFromTeamYaml, teamFromTeamYaml)
				} else {
					fmt.Printf("ERROR: %s in teams.yaml for %s, but NOT in users.yaml\n", userFromTeamYaml, teamFromTeamYaml)
				}
			}
		}

	}
}

func RepoTeamDiff(reposFromRepoYaml map[string]*Repo, teamsFromTeamYaml map[string]*Team, reposTeamsFromGithub map[string]*Repo) {
	// need to add for repos
}

/* Temp save old logic for comparison

	if !teamExists {
		// create a new team
		newTeam := &github.NewTeam{
			Name: teamFromTeamYaml.Name,
		}

		newTeamCreated, _, err := client.Organizations.CreateTeam(ctx, *orgPtr, newTeam)
		if err != nil {
			fmt.Printf("error: %v", err)
		}
		teamFromTeamYaml.Id = newTeamCreated.GetID()
	}

	for _, userFromTeamYaml := range teamFromTeamYaml.Users {
		userFoundInYaml := false
		for _, userFromUserYaml := range u.Users {
			if userFromTeamYaml == userFromUserYaml.GithubUser {
				isMember, _, err := client.Organizations.IsTeamMember(ctx, teamFromTeamYaml.Id, userFromTeamYaml)
				if err != nil {
					fmt.Printf("error: %v", err)
				}
				if !isMember {
					_, _, err := client.Organizations.AddTeamMembership(ctx, teamFromTeamYaml.Id, userFromTeamYaml, nil)

					if err != nil {
						fmt.Printf("error: %v", err)
					}
				}
				userFoundInYaml = true
				break
			}
		}
		if !userFoundInYaml {
			fmt.Printf("ERROR: %s in teams.yaml, but NOT in users.yaml\n", userFromTeamYaml)
		}
	}

}

githubRepos, _, err := client.Repositories.ListByOrg(ctx, *orgPtr, nil)

if err != nil {
	fmt.Println(err)
}

for _, repoFromYaml := range r.Repos {
	repoExists := false
	for _, githubRepo := range githubRepos {
		if githubRepo.GetName() == repoFromYaml.RepoName {
			//team.Id = t1.GetID()
			repoExists = true
			break
		}
	}

	if repoExists {

		githubRepoTeams, _, err := client.Repositories.ListTeams(ctx, *orgPtr, repoFromYaml.RepoName, nil)
		if err != nil {
			fmt.Println(err)
		}

		opts := &github.OrganizationAddTeamRepoOptions{}
		opts.Permission = "pull"

		for _, readTeamFromYaml := range repoFromYaml.Read {
			// TODO - add check if team exists in teams.yaml first?

			for _, githubRepoTeam := range githubRepoTeams {
				teamHasGithubRepoAccess := false
				if githubRepoTeam.GetName() == readTeamFromYaml {
					teamHasGithubRepoAccess = true

					if githubRepoTeam.GetPermission() != opts.Permission {
						_, err = client.Organizations.AddTeamRepo(ctx, githubRepoTeam.GetID(), *orgPtr, repoFromYaml.RepoName, opts)
						if err != nil {
							fmt.Println(err)
						}
					}
				}
				if !teamHasGithubRepoAccess {
					for _, githubTeam := range githubTeams {

						if githubTeam.GetName() == readTeamFromYaml {
							_, err = client.Organizations.AddTeamRepo(ctx, githubTeam.GetID(), *orgPtr, repoFromYaml.RepoName, opts)
							if err != nil {
								fmt.Println(err)
							}
							break
						}
					}
				}
			}

		}

		opts.Permission = "push"
		for _, writeTeamFromYaml := range repoFromYaml.Write {
			// TODO - add check if team exists in teams.yaml first?

			for _, githubRepoTeam := range githubRepoTeams {
				teamHasGithubRepoAccess := false
				if githubRepoTeam.GetName() == writeTeamFromYaml {
					teamHasGithubRepoAccess = true

					if githubRepoTeam.GetPermission() != opts.Permission {
						_, err = client.Organizations.AddTeamRepo(ctx, githubRepoTeam.GetID(), *orgPtr, repoFromYaml.RepoName, opts)
						if err != nil {
							fmt.Println(err)
						}
					}
				}
				if !teamHasGithubRepoAccess {
					for _, githubTeam := range githubTeams {

						if githubTeam.GetName() == writeTeamFromYaml {
							_, err = client.Organizations.AddTeamRepo(ctx, githubTeam.GetID(), *orgPtr, repoFromYaml.RepoName, opts)
							if err != nil {
								fmt.Println(err)
							}
							break
						}
					}
				}
			}

		}

		opts.Permission = "admin"
		for _, adminTeamFromYaml := range repoFromYaml.Admin {
			// TODO - add check if team exists in teams.yaml first?

			for _, githubRepoTeam := range githubRepoTeams {
				teamHasGithubRepoAccess := false
				if githubRepoTeam.GetName() == adminTeamFromYaml {
					teamHasGithubRepoAccess = true

					if githubRepoTeam.GetPermission() != opts.Permission {
						_, err = client.Organizations.AddTeamRepo(ctx, githubRepoTeam.GetID(), *orgPtr, repoFromYaml.RepoName, opts)
						if err != nil {
							fmt.Println(err)
						}
					}
				}
				if !teamHasGithubRepoAccess {
					for _, githubTeam := range githubTeams {

						if githubTeam.GetName() == adminTeamFromYaml {
							_, err = client.Organizations.AddTeamRepo(ctx, githubTeam.GetID(), *orgPtr, repoFromYaml.RepoName, opts)
							if err != nil {
								fmt.Println(err)
							}
							break
						}
					}
				}
			}
		}
	} else {
		fmt.Printf("ERROR: %s in repos.yaml, but DOES NOT exist on GitHub in %s org\n", repoFromYaml.RepoName, *orgPtr)
	}
}
*/
