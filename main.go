package main

import (
        "fmt"
        "log"
        "context"
        "golang.org/x/oauth2"
        "io/ioutil"
        "gopkg.in/yaml.v2"
        "github.com/google/go-github/github"
)

type Users struct {
        Users []struct {
                GithubUser   string   `yaml:"githubuser"`
                ADUser       string   `yaml:"aduser"`
        }
}

type Teams struct {
        Teams []struct {
                Name   string   `yaml:"name"`
                Users  []string `yaml:",flow"`
        }
}

type Repos struct {
        Repos []struct {
                RepoName   string   `yaml:"repoName"`
                Admin      []string `yaml:",flow"`
                Read       []string `yaml:",flow"`
                Write      []string `yaml:",flow"`
        }
}

func main() {
        
        processUsers()
        processTeams()
        processRepos()

}

func processUsers() {

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
	       
    	}

    	// TODO may need to add user to github org ?  
    	// https://developer.github.com/v3/orgs/members/#add-or-update-organization-membership 
    	// PUT /orgs/:org/memberships/:username

    	// it actually looks like adding to a team will send invite to org
    	// https://developer.github.com/v3/teams/members/#add-or-update-team-membership
    	// PUT /teams/:team_id/memberships/:username

    	// TODO maybe add to default splunk team that gets read access to all repos? 

}

func processTeams() {

		//process teams.yaml
        t := Teams{}

        teamsYamlFile, errTest := ioutil.ReadFile("teams.yaml")
    
	    if errTest != nil {
	        log.Printf("teamsYamlFile.Get err   #%v ", errTest)
	    }
    
        err := yaml.Unmarshal(teamsYamlFile, &t)
        if err != nil {
                log.Fatalf("error: %v", err)
        }
        
        for _, team := range t.Teams {
	        fmt.Printf("Name: %s\n", team.Name)

	        // create a new team 
			newteam := &github.NewTeam{
				Name:    team.Name,
			}
			//client := github.NewClient(nil)
			ctx := context.Background()
			
			// personal access token for now
			ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: "..."},
			)

			tc := oauth2.NewClient(ctx, ts)

			client := github.NewClient(tc)

		    // TODO update to take org as parameter
			newteamid, _, err := client.Organizations.CreateTeam(ctx, "speciallll", newteam)

	        // print error (ie. team already exists)
	        // TODO need to get team if already exists 
	        if err != nil {
	                fmt.Printf("error: %v", err)
	        }
        
    	
	        for _, user := range team.Users {
	        	fmt.Printf("User: %s\n", user)
	        	// https://godoc.org/github.com/google/go-github/github#OrganizationsService
	        	// func (s *OrganizationsService) AddTeamMembership(ctx context.Context, team int64, user string, opt *OrganizationAddTeamMembershipOptions) (*Membership, *Response, error)
	        	_, _, err := client.Organizations.AddTeamMembership(ctx, newteamid.GetID(), user, nil)

		        // print error (ie. team already exists)
		        if err != nil {
		                fmt.Printf("error: %v", err)
		        }
	        }

	    }
    	

}

func processRepos() {
		//process repos.yaml
    	r := Repos{}

        reposYamlFile, errTest := ioutil.ReadFile("repos.yaml")
    
	    if errTest != nil {
	        log.Printf("reposYamlFile.Get err   #%v ", errTest)
	    }
    
        err := yaml.Unmarshal(reposYamlFile, &r)
        if err != nil {
                log.Fatalf("error: %v", err)
        }

        for _, repo := range r.Repos {
	        fmt.Printf("Repo Name: %s\n", repo.RepoName)
	        // TODO need to error if repo doesn't exist?  or should it create repo?
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