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

type Users struct {
	Users []struct {
		GithubUser string `yaml:"githubuser"`
		ADUser     string `yaml:"aduser"`
	}
}

type Teams struct {
	Teams []struct {
		Name  string   `yaml:"name"`
		Id    int64    `yaml:"id"`
		Users []string `yaml:",flow"`
	}
}

type Repos struct {
	Repos []struct {
		RepoName string   `yaml:"repoName"`
		Admin    []string `yaml:",flow"`
		Read     []string `yaml:",flow"`
		Write    []string `yaml:",flow"`
	}
}

// TODO STILL NEED TO HANDLE
// if users are removed from teams
// if teams are removed from repos

func main() {

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

	rateLimits, _, err := client.RateLimits(ctx)

	fmt.Printf("Rate Limit:  %d\n", rateLimits.GetCore().Limit)
	fmt.Printf("Remaining Rate Limit:  %d\n", rateLimits.GetCore().Remaining)

	//process users.yaml
	u := Users{}

	usersYamlFile, err := ioutil.ReadFile("users.yaml")

	if err != nil {
		fmt.Printf("usersYamlFile.Get err   #%v ", err)
	}

	err = yaml.Unmarshal(usersYamlFile, &u)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// TODO maybe add to default splunk team that gets read access to all repos?
	/*for _, user := range u.Users {

	}*/

	//process teams.yaml
	t := Teams{}

	teamsYamlFile, err := ioutil.ReadFile("teams.yaml")

	if err != nil {
		fmt.Printf("teamsYamlFile.Get err   #%v ", err)
	}

	err = yaml.Unmarshal(teamsYamlFile, &t)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	githubTeams, _, err := client.Organizations.ListTeams(ctx, *orgPtr, nil)
	if err != nil {
		fmt.Println(err)
	}
	for _, teamFromTeamYaml := range t.Teams {
		teamExists := false

		for _, githubTeam := range githubTeams {
			if githubTeam.GetName() == teamFromTeamYaml.Name {
				teamFromTeamYaml.Id = githubTeam.GetID()
				teamExists = true
				break
			}
		}

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

	//process repos.yaml
	r := Repos{}

	reposYamlFile, err := ioutil.ReadFile("repos.yaml")

	if err != nil {
		fmt.Printf("reposYamlFile.Get err   #%v ", err)
	}

	err = yaml.Unmarshal(reposYamlFile, &r)
	if err != nil {
		log.Fatalf("error: %v", err)
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

}
