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

	yamlTeams, yamlRepos := getDataFromYaml()
	githubTeams, githubRepos := getDataFromGithub()

	TeamDiff(yamlTeams, githubTeams)
	RepoDiff(yamlRepos, githubRepos)

	// Need to add execute (and dry run)
}

func getDataFromYaml() (map[string]*Team, map[string]*Repo) {

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

	for teamName, teamValues := range teams {

		var filteredUsers []string
		for _, user := range teamValues.Users {
			if _, ok := users[user]; !ok {
				fmt.Printf("ERROR: %s in teams.yaml for %s, but NOT in users.yaml\n", user, teamName)
			} else {
				filteredUsers = append(filteredUsers, user)
			}

		}
		teamValues.Users = filteredUsers
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

	for repo, repoValues := range repos {

		var filteredAdmin []string
		for _, adminTeam := range repoValues.Admin {
			if _, ok := teams[adminTeam]; !ok {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", adminTeam, repo)
			} else {
				filteredAdmin = append(filteredAdmin, adminTeam)
			}

		}
		var filteredRead []string
		for _, readTeam := range repoValues.Read {
			if _, ok := teams[readTeam]; !ok {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", readTeam, repo)
			} else {
				filteredRead = append(filteredRead, readTeam)
			}

		}
		var filteredWrite []string
		for _, writeTeam := range repoValues.Write {
			if _, ok := teams[writeTeam]; !ok {
				fmt.Printf("ERROR: %s in repos.yaml for %s, but NOT in teams.yaml\n", writeTeam, repo)
			} else {
				filteredWrite = append(filteredWrite, writeTeam)
			}

		}
		repoValues.Admin = filteredAdmin
		repoValues.Read = filteredRead
		repoValues.Write = filteredWrite
	}

	return teams, repos

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

	teams := make(map[string]*Team)
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
			teams[githubTeam.GetName()] = &t

		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	repos := make(map[string]*Repo)
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

func TeamDiff(yamlTeams map[string]*Team, githubTeams map[string]*Team) {
	for yamlTeamName, yamlTeamValues := range yamlTeams {

		if githubTeam, ok := githubTeams[yamlTeamName]; ok {
			fmt.Printf("Found Match on Github for Team %s\n", yamlTeamName)
			for _, yamlUser := range yamlTeamValues.Users {
				userFoundonGithub := false
				for _, githubUser := range githubTeam.Users {
					if githubUser == yamlUser {
						fmt.Printf("Found Match on Github user %s for Team %s\n", yamlUser, yamlTeamName)
						userFoundonGithub = true
						break
					}
				}
				if !userFoundonGithub {
					fmt.Printf("Need to add member %s to team  %s on Github\n", yamlUser, yamlTeamName)
				}

			}
			for _, githubUser := range githubTeam.Users {
				userFoundinYaml := false
				for _, yamlUser := range yamlTeamValues.Users {
					if githubUser == yamlUser {
						userFoundinYaml = true
						break
					}
				}
				if !userFoundinYaml {
					fmt.Printf("Need to delete %s from team  %s on Github\n", githubUser, yamlTeamName)
				}
			}
		} else {
			fmt.Printf("Need to create team on Github %s\n", yamlTeamName)
			for _, yamlUser := range yamlTeamValues.Users {
				fmt.Printf("Need to add member %s to team  %s on Github\n", yamlUser, yamlTeamName)
			}
		}

	}
}

func RepoDiff(yamlRepos map[string]*Repo, githubRepos map[string]*Repo) {
	for yamlRepoName, yamlRepoValues := range yamlRepos {

		if githubRepo, ok := githubRepos[yamlRepoName]; ok {
			fmt.Printf("Found Match on Github for Repo %s\n", yamlRepoName)

			for _, yamlReadTeam := range yamlRepoValues.Read {
				teamFoundonGithub := false
				for _, githubRead := range githubRepo.Read {
					if githubRead == yamlReadTeam {
						fmt.Printf("Found Match on Github %s for Team %s as READ\n", yamlReadTeam, yamlRepoName)
						teamFoundonGithub = true
						break
					}
				}
				if !teamFoundonGithub {
					fmt.Printf("Need to add team %s as READ to repo  %s on Github\n", yamlReadTeam, yamlRepoName)
				}

			}

			for _, yamlWriteTeam := range yamlRepoValues.Write {
				teamFoundonGithub := false
				for _, githubWrite := range githubRepo.Write {
					if githubWrite == yamlWriteTeam {
						fmt.Printf("Found Match on Github %s for Team %s as WRITE\n", yamlWriteTeam, yamlRepoName)
						teamFoundonGithub = true
						break
					}
				}
				if !teamFoundonGithub {
					fmt.Printf("Need to add team %s as WRITE to repo  %s on Github\n", yamlWriteTeam, yamlRepoName)
				}

			}

			for _, yamlAdminTeam := range yamlRepoValues.Admin {
				teamFoundonGithub := false
				for _, githubAdmin := range githubRepo.Admin {
					if githubAdmin == yamlAdminTeam {
						fmt.Printf("Found Match on Github %s for Team %s as ADMIN\n", yamlAdminTeam, yamlRepoName)
						teamFoundonGithub = true
						break
					}
				}
				if !teamFoundonGithub {
					fmt.Printf("Need to add team %s as ADMIN to repo  %s on Github\n", yamlAdminTeam, yamlRepoName)
				}

			}

			var teams []string
			for _, githubRead := range githubRepo.Read {
				teams = append(teams, githubRead)
			}
			for _, githubWrite := range githubRepo.Write {
				teams = append(teams, githubWrite)
			}
			for _, githubAdmin := range githubRepo.Admin {
				teams = append(teams, githubAdmin)
			}

			for _, githubTeam := range teams {

				teamFoundinYaml := false
				for _, yamlAdminTeam := range yamlRepoValues.Admin {
					if githubTeam == yamlAdminTeam {
						teamFoundinYaml = true
						break
					}
				}
				if !teamFoundinYaml {
					for _, yamlReadTeam := range yamlRepoValues.Read {
						if githubTeam == yamlReadTeam {
							teamFoundinYaml = true
							break
						}
					}
				}
				if !teamFoundinYaml {
					for _, yamlWriteTeam := range yamlRepoValues.Write {
						if githubTeam == yamlWriteTeam {
							teamFoundinYaml = true
							break
						}
					}
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
