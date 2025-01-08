package checkmyrepoCmd

import (
	"fmt"
	"log"
	"sort"

	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/spf13/cobra"
)

var checkmyrepoCmd = &cobra.Command{
	Use:   "check",
	Short: "command to check the code contribution of all developers",
	Run:   cmdRun,
}

func cmdRun(cmd *cobra.Command, args []string) {
	println("in the check command")
	repo, err := git.PlainOpen(".")
	if err != nil {
		cmd.PrintErrln("Error:this is not a git repo")
	}
	ref, err := repo.Head()
	if err != nil {
		log.Fatalf("Failed to get HEAD reference: %v", err)
	}

	commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		log.Fatalf("Failed to get commit history: %v", err)
	}
	authorChanges := make(map[string]int)
	var totalChanges int
	err = commitIter.ForEach(func(c *object.Commit) error {
		// fmt.Printf("Commit: %s\nAuthor: %s <%s>\nDate: %s\nMessage: %s\n\n",
		// 	c.Hash.String(),
		// 	c.Author.Name,
		// 	c.Author.Email,
		// 	c.Author.When,
		// 	c.Message,
		// )

		stats, err := c.Stats()
		if err != nil {
			log.Fatalf("Failed to get Stats: %v", err)
		}
		changes := 0
		for _, stat := range stats {
			changes += stat.Addition + stat.Deletion
		}
		author := fmt.Sprintf("%s <%s>", c.Author.Name, c.Author.Email)
		authorChanges[author] += changes
		totalChanges += changes

		return nil
	})
	if err != nil {
		log.Fatalf("Error while iterating commits: %v", err)
	}

	type AuthorStat struct {
		Author string
		Equity float64
	}
	var stats []AuthorStat
	for author, changes := range authorChanges {
		equity := (float64(changes) / float64(totalChanges)) * 100
		stats = append(stats, AuthorStat{Author: author, Equity: equity})
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Equity > stats[j].Equity
	})
	fmt.Println("Developers of this Repo:")
	for _, stat := range stats {
		redValue := uint8((1 - (stat.Equity / 100)) * 255)
		greenValue := uint8((stat.Equity / 100) * 255)
		// equityColor := color.New().Add(color.Attribute(38)).Add(color.Attribute(48)).
		// 	Add(color.Attribute(5)).
		// 	Add(color.Attribute(16 + int(redValue/36)*36 + int(greenValue/36)*6)).
		// 	Sprint(fmt.Sprintf("%.2f%%", stat.Equity))
		equityColor := color.RGB(int(redValue), int(greenValue), 0).Sprintf(fmt.Sprintf("%.2f%%%%", stat.Equity))
		authorColor := color.New(color.FgMagenta).Sprint(stat.Author)

		fmt.Printf("    Author: %s owns Code Equity: %s\n", authorColor, equityColor)
	}

}

func Init() *cobra.Command {
	return checkmyrepoCmd
}
