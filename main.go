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

	teams, _, err := client.Organizations.ListTeams(ctx, *orgPtr, nil)
	if err != nil {
		fmt.Println(err)
	}
	for _, team := range t.Teams {
		teamExists := false

		for _, t1 := range teams {
			if t1.GetName() == team.Name {
				team.Id = t1.GetID()
				teamExists = true
				break
			}
		}

		if !teamExists {
			// create a new team
			newteam := &github.NewTeam{
				Name: team.Name,
			}

			newteamid, _, err := client.Organizations.CreateTeam(ctx, *orgPtr, newteam)
			if err != nil {
				fmt.Printf("error: %v", err)
			}
			team.Id = newteamid.GetID()
		}

		for _, user := range team.Users {
			userFoundInYaml := false
			for _, userInYaml := range u.Users {
				if user == userInYaml.GithubUser {
					isMember, _, err := client.Organizations.IsTeamMember(ctx, team.Id, user)
					if err != nil {
						fmt.Printf("error: %v", err)
					}
					if !isMember {
						_, _, err := client.Organizations.AddTeamMembership(ctx, team.Id, user, nil)

						if err != nil {
							fmt.Printf("error: %v", err)
						}
					}
					userFoundInYaml = true
					break
				}
			}
			if !userFoundInYaml {
				fmt.Printf("ERROR: %s in teams.yaml, but NOT in users.yaml\n", user)
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

	repos, _, err := client.Repositories.ListByOrg(ctx, *orgPtr, nil)

	if err != nil {
		fmt.Println(err)
	}

	for _, repo := range r.Repos {
		repoExists := false
		for _, r1 := range repos {
			if r1.GetName() == repo.RepoName {
				//team.Id = t1.GetID()
				repoExists = true
				break
			}

		}

		if repoExists {

			repoteams, _, err := client.Repositories.ListTeams(ctx, *orgPtr, repo.RepoName, nil)
			if err != nil {
				fmt.Println(err)
			}

			opts := &github.OrganizationAddTeamRepoOptions{}
			opts.Permission = "pull"

			for _, readTeam := range repo.Read {
				// TODO - add check if team exists in teams.yaml first?

				for _, repoteam := range repoteams {
					teamHasRepoAccess := false
					if repoteam.GetName() == readTeam {
						teamHasRepoAccess = true

						if repoteam.GetPermission() != opts.Permission {
							_, err = client.Organizations.AddTeamRepo(ctx, repoteam.GetID(), *orgPtr, repo.RepoName, opts)
							if err != nil {
								fmt.Println(err)
							}
						}
					}
					if !teamHasRepoAccess {
						for _, t2 := range teams {

							if t2.GetName() == readTeam {
								_, err = client.Organizations.AddTeamRepo(ctx, t2.GetID(), *orgPtr, repo.RepoName, opts)
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
			for _, writeTeam := range repo.Write {
				// TODO - add check if team exists in teams.yaml first?

				for _, repoteam := range repoteams {
					teamHasRepoAccess := false
					if repoteam.GetName() == writeTeam {
						teamHasRepoAccess = true

						if repoteam.GetPermission() != opts.Permission {
							_, err = client.Organizations.AddTeamRepo(ctx, repoteam.GetID(), *orgPtr, repo.RepoName, opts)
							if err != nil {
								fmt.Println(err)
							}
						}
					}
					if !teamHasRepoAccess {
						for _, t2 := range teams {

							if t2.GetName() == writeTeam {
								_, err = client.Organizations.AddTeamRepo(ctx, t2.GetID(), *orgPtr, repo.RepoName, opts)
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
			for _, adminTeam := range repo.Admin {
				// TODO - add check if team exists in teams.yaml first?

				for _, repoteam := range repoteams {
					teamHasRepoAccess := false
					if repoteam.GetName() == adminTeam {
						teamHasRepoAccess = true

						if repoteam.GetPermission() != opts.Permission {
							_, err = client.Organizations.AddTeamRepo(ctx, repoteam.GetID(), *orgPtr, repo.RepoName, opts)
							if err != nil {
								fmt.Println(err)
							}
						}
					}
					if !teamHasRepoAccess {
						for _, t2 := range teams {

							if t2.GetName() == adminTeam {
								_, err = client.Organizations.AddTeamRepo(ctx, t2.GetID(), *orgPtr, repo.RepoName, opts)
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
			fmt.Printf("ERROR: %s in repos.yaml, but DOES NOT exist on GitHub in %s org\n", repo.RepoName, *orgPtr)
		}
	}

}
