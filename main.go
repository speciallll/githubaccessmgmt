package main

import (
	"context"
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

func main() {

	//setup github client
	ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: "..."},
	) //personal access token for now

	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	// TODO The GitHub client gives you info about the current rate limit
	// let’s make sure we print that, at least to understand.

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
		fmt.Printf("GitHub UserName: %s\n", user.GithubUser)

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

	opts := &github.ListOptions{}
	teams, _, err := client.Organizations.ListTeams(ctx, "speciallll", opts) // TODO update to take org as parameter
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

			// TODO update to take org as parameter
			newteamid, _, err := client.Organizations.CreateTeam(ctx, "speciallll", newteam)
			team.Id = newteamid.GetID()

			if err != nil {
				fmt.Printf("error: %v", err)
			}
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

	for _, repo := range r.Repos {
		fmt.Printf("Repo Name: %s\n", repo.RepoName)
		// TODO need to error if repo doesn't exist on github?
		// or should it create repo?  prob not for now
		for _, adminTeams := range repo.Admin {
			fmt.Printf("Admin Teams: %s\n", adminTeams)
			// TODO need to add team as admin to repo
		}
		for _, readTeams := range repo.Read {
			fmt.Printf("Read Teams: %s\n", readTeams)
			// TODO need to add team as read to repo
		}
		for _, writeTeams := range repo.Write {
			fmt.Printf("Write Teams: %s\n", writeTeams)
			// TODO need to add team as write to repo
		}
	}

}
