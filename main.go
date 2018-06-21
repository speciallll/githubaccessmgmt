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

	//process users.yaml
	u := Users{}

	usersYamlFile, errTest := ioutil.ReadFile("users.yaml")

	if errTest != nil {
		log.Printf("usersYamlFile.Get err   #%v ", errTest)
	}

	err := yaml.Unmarshal(usersYamlFile, &u)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	for _, user := range u.Users {
		fmt.Printf("GitHub UserName: %s\n", user.GithubUser)
		// TODO maybe add to default splunk team that gets read access to all repos?
	}

	//process teams.yaml
	t := Teams{}

	teamsYamlFile, errTest := ioutil.ReadFile("teams.yaml")

	if errTest != nil {
		log.Printf("teamsYamlFile.Get err   #%v ", errTest)
	}

	err = yaml.Unmarshal(teamsYamlFile, &t)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	opts := &github.ListOptions{}
	teams, _, err := client.Organizations.ListTeams(ctx, "speciallll", opts)
	if err != nil {
		fmt.Println(err)
	}
	for _, team := range t.Teams {
		fmt.Printf("Name: %s\n", team.Name)
		teamExists := false

		for _, t1 := range teams {
			if t1.GetName() == team.Name {
				fmt.Printf("Team %q has ID %d\n", team.Name, t1.GetID())
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
			fmt.Printf("User: %s\n", user)
			for _, userInYaml := range u.Users {
				// TODO check if membership already exists first
				if user == userInYaml.GithubUser {
					_, _, err := client.Organizations.AddTeamMembership(ctx, team.Id, user, nil)

					if err != nil {
						fmt.Printf("error: %v", err)
					}
					break
				}
			}
		}

	}

	//process repos.yaml
	r := Repos{}

	reposYamlFile, errTest := ioutil.ReadFile("repos.yaml")

	if errTest != nil {
		log.Printf("reposYamlFile.Get err   #%v ", errTest)
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
