// package checkmyrepoCmd

// import (
// 	"fmt"
// 	"log"
// 	"sort"

// 	"github.com/fatih/color"
// 	"github.com/go-git/go-git/v5"
// 	"github.com/go-git/go-git/v5/plumbing/object"
// 	"github.com/spf13/cobra"
// )

// var checkmyrepoCmd = &cobra.Command{
// 	Use:   "check",
// 	Short: "command to check the code contribution of all developers",
// 	Run:   cmdRun,
// }

// func cmdRun(cmd *cobra.Command, args []string) {
// 	repo, err := git.PlainOpen(".")
// 	if err != nil {
// 		cmd.PrintErrln("Error:this is not a git repo")
// 	}
// 	ref, err := repo.Head()
// 	if err != nil {
// 		log.Fatalf("Failed to get HEAD reference: %v", err)
// 	}

// 	commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
// 	if err != nil {
// 		log.Fatalf("Failed to get commit history: %v", err)
// 	}
// 	authorChanges := make(map[string]int)
// 	var totalChanges int
// 	err = commitIter.ForEach(func(c *object.Commit) error {
// 		// fmt.Printf("Commit: %s\nAuthor: %s <%s>\nDate: %s\nMessage: %s\n\n",
// 		// 	c.Hash.String(),
// 		// 	c.Author.Name,
// 		// 	c.Author.Email,
// 		// 	c.Author.When,
// 		// 	c.Message,
// 		// )

// 		stats, err := c.Stats()
// 		if err != nil {
// 			log.Fatalf("Failed to get Stats: %v", err)
// 		}
// 		changes := 0
// 		for _, stat := range stats {
// 			changes += stat.Addition + stat.Deletion
// 		}
// 		author := fmt.Sprintf("%s <%s>", c.Author.Name, c.Author.Email)
// 		authorChanges[author] += changes
// 		totalChanges += changes

// 		return nil
// 	})
// 	if err != nil {
// 		log.Fatalf("Error while iterating commits: %v", err)
// 	}

// 	type AuthorStat struct {
// 		Author string
// 		Equity float64
// 	}
// 	var stats []AuthorStat
// 	for author, changes := range authorChanges {
// 		equity := (float64(changes) / float64(totalChanges)) * 100
// 		stats = append(stats, AuthorStat{Author: author, Equity: equity})
// 	}
// 	sort.Slice(stats, func(i, j int) bool {
// 		return stats[i].Equity > stats[j].Equity
// 	})
// 	fmt.Println("Developers of this Repo:")
// 	for _, stat := range stats {
// 		redValue := uint8((1 - (stat.Equity / 100)) * 255)
// 		greenValue := uint8((stat.Equity / 100) * 255)
// 		// equityColor := color.New().Add(color.Attribute(38)).Add(color.Attribute(48)).
// 		// 	Add(color.Attribute(5)).
// 		// 	Add(color.Attribute(16 + int(redValue/36)*36 + int(greenValue/36)*6)).
// 		// 	Sprint(fmt.Sprintf("%.2f%%", stat.Equity))
// 		equityColor := color.RGB(int(redValue), int(greenValue), 0).Sprintf(fmt.Sprintf("%.2f%%%%", stat.Equity))
// 		authorColor := color.New(color.FgMagenta).Sprint(stat.Author)

// 		fmt.Printf("    Author: %s owns Code Equity: %s\n", authorColor, equityColor)
// 	}

// }

// func Init() *cobra.Command {
// 	return checkmyrepoCmd
// }

package checkmyrepoCmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/utils/diff"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/spf13/cobra"
)

var (
	username string
	password string
)

var checkmyrepoCmd = &cobra.Command{
	Use:   "check [repository URL]",
	Short: "command to check the code contribution of all developers",
	Args:  cobra.MaximumNArgs(1),
	Run:   cmdRun,
	Example: `  checkmyrepo check                               # Check local repository
  checkmyrepo check https://github.com/user/repo     # Check remote repository
  checkmyrepo check https://gitlab.com/user/repo     # Check GitLab repository
  checkmyrepo check https://bitbucket.org/user/repo  # Check Bitbucket repository
  checkmyrepo check github.user/repo                 # Check remote repository
  checkmyrepo check gitlab.user/repo                 # Check GitLab repository
  checkmyrepo check bitbucket.user/repo              # Check Bitbucket repository`,
}

func getRepo(repoURL string) (*git.Repository, string, error) {
	var auth *http.BasicAuth
	if username != "" && password != "" {
		auth = &http.BasicAuth{
			Username: username,
			Password: password,
		}
	}
	if repoURL == "" {
		repo, err := git.PlainOpen(".")
		return repo, "", err
	}
	if !strings.Contains(repoURL, "://") {
		parts := strings.Split(repoURL, "/")
		if len(parts) != 2 {
			return nil, "", fmt.Errorf("invalid repository format. Use 'owner/repo' or full URL")
		}
		switch {
		case strings.Contains(parts[0], "gitlab."):
			group := strings.Split(parts[0], ".")[1]
			repoURL = fmt.Sprintf("https://gitlab.com/%s/%s", group, parts[1])
		case strings.Contains(parts[0], "bitbucket."):
			username := strings.Split(parts[0], ".")[1]
			repoURL = fmt.Sprintf("https://bitbucket.org/%s/%s", username, parts[1])
		case strings.Contains(parts[0], "github."):
			username := strings.Split(parts[0], ".")[1]
			repoURL = fmt.Sprintf("https://github.com/%s/%s", username, parts[1])
		default:
			repoURL = "https://github.com/" + repoURL
		}
	}
	tempDir, err := os.MkdirTemp(".", "repo-*")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp directory: %v", err)
	}

	fmt.Printf("Cloning repository %s...\n", repoURL)
	repo, err := git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:      repoURL,
		Progress: os.Stdout,
		Auth:     auth,
	})
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, "", fmt.Errorf("failed to clone repository: %v", err)
	}

	return repo, tempDir, nil
}

// func cmdRun(cmd *cobra.Command, args []string) {
// 	var repoURL string
// 	if len(args) > 0 {
// 		repoURL = args[0]
// 	}

// 	repo, tempDir, err := getRepo(repoURL)
// 	if err != nil {
// 		cmd.PrintErrf("Error: %v\n", err)
// 		return
// 	}
// 	if tempDir != "" {
// 		defer os.RemoveAll(tempDir)
// 	}

// 	ref, err := repo.Head()
// 	if err != nil {
// 		log.Fatalf("Failed to get HEAD reference: %v", err)
// 	}

// 	commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
// 	if err != nil {
// 		log.Fatalf("Failed to get commit history: %v", err)
// 	}

// 	authorChanges := make(map[string]int)
// 	var totalChanges int

// 	err = commitIter.ForEach(func(c *object.Commit) error {
// 		// fmt.Printf("Commit: %s\nAuthor: %s <%s>\nDate: %s\nMessage: %s\n\n",
// 		// 	c.Hash.String(),
// 		// 	c.Author.Name,
// 		// 	c.Author.Email,
// 		// 	c.Author.When,
// 		// 	c.Message,
// 		// )
// 		stats, err := c.Stats()
// 		if err != nil {
// 			log.Fatalf("Failed to get Stats: %v", err)
// 		}

// 		changes := 0
// 		for _, stat := range stats {
// 			changes += stat.Addition + stat.Deletion
// 		}
// 		author := fmt.Sprintf("%s <%s>", c.Author.Name, c.Author.Email)
// 		authorChanges[author] += changes
// 		totalChanges += changes
// 		return nil
// 	})
// 	if err != nil {
// 		log.Fatalf("Error while iterating commits: %v", err)
// 	}

// 	type AuthorStat struct {
// 		Author string
// 		Equity float64
// 	}

// 	var stats []AuthorStat
// 	for author, changes := range authorChanges {
// 		equity := (float64(changes) / float64(totalChanges)) * 100
// 		stats = append(stats, AuthorStat{Author: author, Equity: equity})
// 	}

// 	sort.Slice(stats, func(i, j int) bool {
// 		return stats[i].Equity > stats[j].Equity
// 	})

// 	if repoURL == "" {
// 		fmt.Println("Developers of local repository:")
// 	} else {
// 		fmt.Printf("Developers of repository %s:\n", repoURL)
// 	}

//		for _, stat := range stats {
//			redValue := uint8((1 - (stat.Equity / 100)) * 255)
//			greenValue := uint8((stat.Equity / 100) * 255)
//			// equityColor := color.New().Add(color.Attribute(38)).Add(color.Attribute(48)).
//			// 	Add(color.Attribute(5)).
//			// 	Add(color.Attribute(16 + int(redValue/36)*36 + int(greenValue/36)*6)).
//			// 	Sprint(fmt.Sprintf("%.2f%%", stat.Equity))
//			equityColor := color.RGB(int(redValue), int(greenValue), 0).Sprintf(fmt.Sprintf("%.2f%%%%", stat.Equity))
//			authorColor := color.New(color.FgMagenta).Sprint(stat.Author)
//			fmt.Printf(" Author: %s owns Code Equity: %s\n", authorColor, equityColor)
//		}
//	}

type AuthorStat struct {
	Author string
	Equity float64
}
type FileHistory struct {
	Path    string
	Commits []*object.Commit
}

// RepositoryHistory analyzes all files in the repository starting from the given commit
// Returns a map of file paths to their commit histories
func references(c *object.Commit, paths []string) (map[string][]*object.Commit, error) {
	result := make(map[string][]*object.Commit)
	seen := make(map[plumbing.Hash]struct{})

	if err := walkGraph(&result, &seen, c, paths, true); err != nil {
		return nil, err
	}

	// TODO result should be returned without ordering
	for _, commits := range result {
		sortCommits(commits) // Sort each list of commits
	}
	var err error
	// Handle merges of identical cherry-picks
	fmt.Println(len(result))
	for path, commits := range result {
		if path == "LICENSE" {
			fmt.Println(len(commits))
			_, err := commits[0].File(path)
			if err != nil {
				fmt.Println("heree how the fuck")
			}
		}
		result[path], err = removeComp(path, commits, equivalent)
		if path == "LICENSE" {
			fmt.Println(len(result[path]))
		}
		if err != nil {
			return nil, err // Return error if removeComp fails
		}
	}

	return result, nil
}

func walkGraph(result *map[string][]*object.Commit, seen *map[plumbing.Hash]struct{}, current *object.Commit, paths []string, firstRun bool) error {
	// check and update seen
	if _, ok := (*seen)[current.Hash]; ok {
		return nil
	}
	(*seen)[current.Hash] = struct{}{}

	// Get all parents that contain any of the paths
	parentsMap, err := parentsContainingPaths(paths, current)
	if err != nil {
		return err
	}

	// Process each path separately
	for _, path := range paths {
		parents := parentsMap[path]

		// If the path does not exist in the current commit, skip it
		if firstRun {
			(*result)[path] = append((*result)[path], current)
		} else if len(parents) == 0 {
			continue
		}

		switch len(parents) {
		case 0:
			// Path was created in this commit
			(*result)[path] = append((*result)[path], current)
		case 1:
			// Check if file contents changed
			different, err := differentContents(path, current, parents)
			if err != nil {
				return err
			}
			if len(different) == 1 {
				(*result)[path] = append((*result)[path], current)
			}
			// Walk the parent
			err = walkGraph(result, seen, parents[0], paths, false)
			if err != nil {
				return err
			}
		default:
			// More than one parent contains the path (possible merge)
			for _, p := range parents {
				err := walkGraph(result, seen, p, paths, false)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// fileChanged checks if a file's contents changed between two commits
func fileChanged(path string, current, parent *object.Commit) bool {
	currentHash, foundCurrent := blobHash(path, current)
	parentHash, foundParent := blobHash(path, parent)

	if !foundCurrent || !foundParent {
		return true
	}

	return currentHash != parentHash
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
func parentsContainingPaths(paths []string, c *object.Commit) (map[string][]*object.Commit, error) {
	result := make(map[string][]*object.Commit)

	// Initialize result map with empty slices for each path
	for _, path := range paths {
		// fmt.Println(path)
		result[path] = []*object.Commit{}
	}

	iter := c.Parents()
	for {
		parent, err := iter.Next()
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return nil, err
		}

		// Create a set of all files in the parent commit
		fileSet := make(map[string]struct{})
		tree, err := parent.Tree()
		if err != nil {
			return nil, err
		}
		err = tree.Files().ForEach(func(f *object.File) error {
			fileSet[f.Name] = struct{}{}
			return nil
		})
		if err != nil {
			return nil, err
		}

		// Check if any of the required paths exist in the fileSet
		for _, path := range paths {
			if _, exists := fileSet[path]; exists {
				result[path] = append(result[path], parent)
			}
			// fmt.Printf("File: %s, Parents found: %d\n", path, len(result[path]))

		}
	}
}

type commitSorterer struct {
	l []*object.Commit
}

func (s commitSorterer) Len() int {
	return len(s.l)
}

func (s commitSorterer) Less(i, j int) bool {
	return s.l[i].Committer.When.Before(s.l[j].Committer.When) ||
		s.l[i].Committer.When.Equal(s.l[j].Committer.When) &&
			s.l[i].Author.When.Before(s.l[j].Author.When)
}

func (s commitSorterer) Swap(i, j int) {
	s.l[i], s.l[j] = s.l[j], s.l[i]
}

// SortCommits sorts a commit list by commit date, from older to newer.
func sortCommits(l []*object.Commit) {
	s := &commitSorterer{l}
	sort.Sort(s)
}

// Returns an slice of the commits in "cs" that has the file "path", but with different
// contents than what can be found in "c".
func differentContents(path string, c *object.Commit, cs []*object.Commit) ([]*object.Commit, error) {
	result := make([]*object.Commit, 0, len(cs))
	h, found := blobHash(path, c)
	if !found {
		return nil, object.ErrFileNotFound
	}
	for _, cx := range cs {
		if hx, found := blobHash(path, cx); found && h != hx {
			result = append(result, cx)
		}
	}
	return result, nil
}

// blobHash returns the hash of a path in a commit
func blobHash(path string, commit *object.Commit) (hash plumbing.Hash, found bool) {
	file, err := commit.File(path)
	if err != nil {
		var empty plumbing.Hash
		return empty, found
	}
	return file.Hash, true
}

type contentsComparatorFn func(path string, a, b *object.Commit) (bool, error)

// Returns a new slice of commits, with duplicates removed.  Expects a
// sorted commit list.  Duplication is defined according to "comp".  It
// will always keep the first commit of a series of duplicated commits.
func removeComp(path string, cs []*object.Commit, comp contentsComparatorFn) ([]*object.Commit, error) {
	result := make([]*object.Commit, 0, len(cs))
	if len(cs) == 0 {
		return result, nil
	}
	result = append(result, cs[0])
	for i := 1; i < len(cs); i++ {
		equals, err := comp(path, cs[i], cs[i-1])
		if err != nil {
			return nil, err
		}
		if !equals {
			result = append(result, cs[i])
		}
	}
	return result, nil
}

// Equivalent commits are commits whose patch is the same.
func equivalent(path string, a, b *object.Commit) (bool, error) {
	numParentsA := a.NumParents()
	numParentsB := b.NumParents()

	// the first commit is not equivalent to anyone
	// and "I think" merges can not be equivalent to anything
	if numParentsA != 1 || numParentsB != 1 {
		return false, nil
	}

	diffsA, err := patch(a, path)
	if err != nil {
		return false, err
	}
	diffsB, err := patch(b, path)
	if err != nil {
		return false, err
	}

	return sameDiffs(diffsA, diffsB), nil
}

func patch(c *object.Commit, path string) ([]diffmatchpatch.Diff, error) {
	// get contents of the file in the commit
	file, err := c.File(path)
	if err != nil {
		return nil, err
	}
	content, err := file.Contents()
	if err != nil {
		return nil, err
	}

	// get contents of the file in the first parent of the commit
	var contentParent string
	iter := c.Parents()
	parent, err := iter.Next()
	if err != nil {
		return nil, err
	}
	file, err = parent.File(path)
	if err != nil {
		contentParent = ""
	} else {
		contentParent, err = file.Contents()
		if err != nil {
			return nil, err
		}
	}

	// compare the contents of parent and child
	return diff.Do(content, contentParent), nil
}

func sameDiffs(a, b []diffmatchpatch.Diff) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameDiff(a[i], b[i]) {
			return false
		}
	}
	return true
}

func sameDiff(a, b diffmatchpatch.Diff) bool {
	if a.Type != b.Type {
		return false
	}
	switch a.Type {
	case 0:
		return countLines(a.Text) == countLines(b.Text)
	case 1, -1:
		return a.Text == b.Text
	default:
		panic("unreachable")
	}
}
func newLine(author, text string, date time.Time, hash plumbing.Hash) *git.Line {
	return &git.Line{
		Author: author,
		Text:   text,
		Hash:   hash,
		Date:   date,
	}
}
func newLines(contents []string, commits []*object.Commit) ([]*git.Line, error) {
	lcontents := len(contents)
	lcommits := len(commits)
	fmt.Printf("Contents: %d, Commits: %d\n", len(contents), len(commits))

	if lcontents != lcommits {
		if lcontents == lcommits-1 && contents[lcontents-1] != "\n" {
			contents = append(contents, "\n")
		} else {
			return nil, fmt.Errorf("contents and commits have different length")
		}
	}

	result := make([]*git.Line, 0, lcontents)
	for i := range contents {
		result = append(result, newLine(
			commits[i].Author.Email, contents[i],
			commits[i].Author.When, commits[i].Hash,
		))
	}

	return result, nil
}

// mapFileRevisions maps file paths to their revision chains by traversing commit history once.
func mapFileRevisions(repo *git.Repository, commit *object.Commit) (map[string][]*object.Commit, error) {
	var filePaths []string

	// Iterate through files and collect file paths
	fileIter, err := commit.Files()
	if err != nil {
		return nil, err
	}

	err = fileIter.ForEach(func(file *object.File) error {
		// fmt.Println(file.Name)
		filePaths = append(filePaths, file.Name)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Ensure file paths are not empty
	if len(filePaths) == 0 {
		return nil, errors.New("no file paths found in the commit")
	}
	fmt.Println(len(filePaths))
	// Process file paths using referencesParallel
	fileRevs, err := references(commit, filePaths)
	fmt.Println(len(fileRevs))
	if err != nil {
		return nil, err
	}
	// fileRevs := make(map[string][]*object.Commit)
	// fileIter, err := commit.Files()
	// var filePaths []string
	// err = fileIter.ForEach(func(file *object.File) error {
	// 	filePaths = append(filePaths, file.Name)
	// 	// filePath := file.Name
	// 	// fileRevs[filePath] = append(fileRevs[filePath], c)
	// 	// fileRevs[filePath], err = references(commit, filePath)
	// 	return nil
	// })
	// fileRevs, err = referencesParallel(commit, filePaths)
	// if err != nil {
	// 	return nil, err
	// }
	// Traverse commit history
	// ref, err := repo.Head()
	// commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	// if err != nil {
	// 	return nil, err
	// }

	// err = commitIter.ForEach(func(c *object.Commit) error {
	// 	tree, err := c.Tree()
	// 	if err != nil {
	// 		return err
	// 	}

	// 	// Iterate over all files in the tree
	// 	return tree.Files().ForEach(func(file *object.File) error {
	// 		filePath := file.Name
	// 		// fileRevs[filePath] = append(fileRevs[filePath], c)
	// 		fileRevs[filePath], err = references(c, filePath)
	// 		return nil
	// 	})
	// })

	return fileRevs, err
}

type blame struct {
	// the path of the file to blame
	path string
	// the commit of the final revision of the file to blame
	fRev *object.Commit
	// the chain of revisions affecting the the file to blame
	revs []*object.Commit
	// the contents of the file across all its revisions
	data []string
	// the graph of the lines in the file across all the revisions
	graph [][]*object.Commit
}

func countLines(s string) int {
	if s == "" {
		return 0
	}

	nEOL := strings.Count(s, "\n")
	if strings.HasSuffix(s, "\n") {
		return nEOL
	}

	return nEOL + 1
}

// calculate the history of a file "path", starting from commit "from", sorted by commit date.
// build graph of a file from its revision history
func (b *blame) fillGraphAndData() error {
	//TODO: not all commits are needed, only the current rev and the prev
	b.graph = make([][]*object.Commit, len(b.revs))
	b.data = make([]string, len(b.revs)) // file contents in all the revisions
	// for every revision of the file, starting with the first
	// one...
	for i, rev := range b.revs {
		// get the contents of the file
		// fmt.Println(i, b.path, rev)
		// fmt.Println(rev.Author)
		file, err := rev.File(b.path)
		if err != nil {
			return nil
		}
		b.data[i], err = file.Contents()
		if err != nil {
			return err
		}
		nLines := countLines(b.data[i])
		// create a node for each line
		b.graph[i] = make([]*object.Commit, nLines)
		// assign a commit to each node
		// if this is the first revision, then the node is assigned to
		// this first commit.
		if i == 0 {
			for j := 0; j < nLines; j++ {
				b.graph[i][j] = b.revs[i]
			}
		} else {
			// if this is not the first commit, then assign to the old
			// commit or to the new one, depending on what the diff
			// says.
			b.assignOrigin(i, i-1)
		}
	}
	return nil
}

// sliceGraph returns a slice of commits (one per line) for a particular
// revision of a file (0=first revision).
func (b *blame) sliceGraph(i int) []*object.Commit {
	fVs := b.graph[i]
	result := make([]*object.Commit, 0, len(fVs))
	for _, v := range fVs {
		c := *v
		result = append(result, &c)
	}
	return result
}

// Assigns origin to vertexes in current (c) rev from data in its previous (p)
// revision
func (b *blame) assignOrigin(c, p int) {
	// assign origin based on diff info
	hunks := diff.Do(b.data[p], b.data[c])
	sl := -1 // source line
	dl := -1 // destination line
	for h := range hunks {
		hLines := countLines(hunks[h].Text)
		for hl := 0; hl < hLines; hl++ {
			switch {
			case hunks[h].Type == 0:
				sl++
				dl++
				b.graph[c][dl] = b.graph[p][sl]
			case hunks[h].Type == 1:
				dl++
				b.graph[c][dl] = b.revs[c]
			case hunks[h].Type == -1:
				sl++
			default:
				panic("unreachable")
			}
		}
	}
}

// GoString prints the results of a Blame using git-blame's style.
func (b *blame) GoString() string {
	var buf bytes.Buffer

	file, err := b.fRev.File(b.path)
	if err != nil {
		panic("PrettyPrint: internal error in repo.Data")
	}
	contents, err := file.Contents()
	if err != nil {
		panic("PrettyPrint: internal error in repo.Data")
	}

	lines := strings.Split(contents, "\n")
	// max line number length
	mlnl := len(strconv.Itoa(len(lines)))
	// max author length
	mal := b.maxAuthorLength()
	format := fmt.Sprintf("%%s (%%-%ds %%%dd) %%s\n",
		mal, mlnl)

	fVs := b.graph[len(b.graph)-1]
	for ln, v := range fVs {
		fmt.Fprintf(&buf, format, v.Hash.String()[:8],
			prettyPrintAuthor(fVs[ln]), ln+1, lines[ln])
	}
	return buf.String()
}

// utility function to pretty print the author.
func prettyPrintAuthor(c *object.Commit) string {
	return fmt.Sprintf("%s %s", c.Author.Name, c.Author.When.Format("2006-01-02"))
}

// utility function to calculate the number of runes needed
// to print the longest author name in the blame of a file.
func (b *blame) maxAuthorLength() int {
	memo := make(map[plumbing.Hash]struct{}, len(b.graph)-1)
	fVs := b.graph[len(b.graph)-1]
	m := 0
	for ln := range fVs {
		if _, ok := memo[fVs[ln].Hash]; ok {
			continue
		}
		memo[fVs[ln].Hash] = struct{}{}
		m = max(m, utf8.RuneCountInString(prettyPrintAuthor(fVs[ln])))
	}
	return m
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// BlameAllFiles processes blame for all files in the repository from a given commit.
func BlameAllFiles(repo *git.Repository, commit *object.Commit) (map[string]*git.BlameResult, error) {
	// Map of file paths to their blame results
	blameResults := make(map[string]*git.BlameResult)

	// Get revision chains for all files
	fileRevs, err := mapFileRevisions(repo, commit)
	if err != nil {
		return nil, err
	}

	// Collect all file paths in the commit
	filesIter, err := commit.Files()
	if err != nil {
		return nil, err
	}

	var filePaths []string
	err = filesIter.ForEach(func(file *object.File) error {
		filePaths = append(filePaths, file.Name)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Process blame for each file
	for _, path := range filePaths {
		fmt.Printf("Processing file: %s\n", path)

		// Use existing revisions if present, otherwise set an empty slice
		revs := fileRevs[path]
		fmt.Println(len(revs))
		blame := &blame{
			path: path,
			fRev: commit,
			revs: revs,
		}
		fmt.Println(len(blame.revs))
		// Fill graph and data
		if err := blame.fillGraphAndData(); err != nil {
			return nil, err
		}

		file, err := commit.File(path)
		if err != nil {
			return nil, err
		}
		finalLines, err := file.Lines()
		if err != nil {
			return nil, err
		}

		lines, err := newLines(finalLines, blame.sliceGraph(len(blame.graph)-1))
		if err != nil {
			return nil, err
		}

		blameResults[path] = &git.BlameResult{
			Path:  path,
			Rev:   commit.Hash,
			Lines: lines,
		}
	}

	return blameResults, nil
}

func cmdRun(cmd *cobra.Command, args []string) {
	// Get repository
	var repoURL string
	if len(args) > 0 {
		repoURL = args[0]
	}

	repo, tempDir, err := getRepo(repoURL)
	if err != nil {
		cmd.PrintErrf("Error: %v", err)
		return
	}
	if tempDir != "" {
		defer os.RemoveAll(tempDir)
	}

	// Get HEAD commit
	ref, err := repo.Head()
	if err != nil {
		log.Fatalf("Failed to get HEAD: %v", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		log.Fatalf("Failed to get commit object: %v", err)
	}

	// Aggregate blame results
	authorLines := make(map[string]int)
	var totalLines int

	blameResults, err := BlameAllFiles(repo, commit)
	if err != nil {
		log.Fatalf("Failed to calculate blame: %v", err)
	}

	for _, blame := range blameResults {
		for _, line := range blame.Lines {
			author := fmt.Sprintf("%s <%s>", line.AuthorName, line.Author)
			if len(strings.TrimSpace(line.Text)) > 0 {
				authorLines[author]++
				totalLines++
			}
		}
	}
	fmt.Println(totalLines)
	// Prepare statistics
	type AuthorStat struct {
		Author string
		Equity float64
	}
	var stats []AuthorStat

	for author, currentLines := range authorLines {
		currentEquity := float64(0)
		if totalLines > 0 {
			currentEquity = (float64(currentLines) / float64(totalLines)) * 100
		}
		stats = append(stats, AuthorStat{
			Author: author,
			Equity: currentEquity,
		})
	}

	// Sort and display stats
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Equity > stats[j].Equity
	})

	for _, stat := range stats {
		redValue := uint8((1 - (stat.Equity / 100)) * 255)
		greenValue := uint8((stat.Equity / 100)) * 255
		equityColor := color.RGB(int(redValue), int(greenValue), 0).Sprintf(fmt.Sprintf("%.2f%%%%", stat.Equity))
		authorColor := color.New(color.FgMagenta).Sprint(stat.Author)
		fmt.Printf("    Author: %s owns Code Equity: %s\n", authorColor, equityColor)
	}
}
func Init() *cobra.Command {
	checkmyrepoCmd.Flags().StringVarP(&username, "username", "u", "", "Username for private repository")
	checkmyrepoCmd.Flags().StringVarP(&password, "password", "p", "", "Password or token for private repository")
	return checkmyrepoCmd
}
