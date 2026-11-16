package state

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/wvanlint/twf/internal/filetree"
)

type State struct {
	Root      *filetree.FileTree
	Cursor    *filetree.FileTree
	Selection []*filetree.FileTree
}

func (s *State) LocatePath(path string) error {
	var node *filetree.FileTree
	var err error
	if filetree.HasWildcard(path) && !s.existsLiterally(path) {
		// "A/B/C/*.pdf": show only the matching entries and start on the first.
		node, err = s.Root.FilterGlob(path)
	} else {
		node, err = s.Root.FindPath(path)
	}
	if err != nil {
		fmt.Println("Error in LocatePath")
		return err
	}
	if !node.Visible() {
		// The located file is hidden by the filter, which is thus dropped.
		s.Root.SetFilter("")
	}
	s.Cursor = node
	for node.Parent() != nil {
		node = node.Parent()
		err = node.Expand()
		if err != nil {
			return err
		}
	}
	return nil
}

// SetFilter hides the files not matching the pattern, replacing any earlier
// filter. The cursor moves up to its directory if it got hidden.
func (s *State) SetFilter(pattern string) error {
	if err := s.Root.SetFilter(pattern); err != nil {
		return err
	}
	for !s.Cursor.Visible() {
		s.Cursor = s.Cursor.Parent()
	}
	return nil
}

// existsLiterally reports whether the path names an existing file as is, so
// that files with names like "foo[1].pdf" are not treated as patterns.
func (s *State) existsLiterally(path string) bool {
	if !filepath.IsAbs(path) {
		path = filepath.Join(s.Root.AbsPath, path)
	}
	_, err := os.Lstat(path)
	return err == nil
}

func (s *State) AutoExpand(maxDepth int, ignore *regexp.Regexp) error {
	return s.Root.Traverse(true, nil, func(tree *filetree.FileTree, depth int) error {
		if maxDepth >= 0 && depth >= maxDepth {
			return nil
		}
		if tree == s.Root {
			return tree.Expand()
		}
		parent := tree.Parent()
		if parent != nil && !parent.Expanded() {
			return nil
		}

		if ignore != nil {
			rel, err := filepath.Rel(s.Root.AbsPath, tree.AbsPath)
			if err != nil {
				return err
			}
			if ignore.MatchString(rel) {
				return nil
			}
		}
		return tree.Expand()
	})
}
