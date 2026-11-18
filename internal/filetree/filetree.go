package filetree

import (
	"errors"
	"fmt"
	"os"
    "strings"
	"path/filepath"
	"sort"
)

type FileTree struct {
	AbsPath        string
	info           os.FileInfo
	targetInfo     os.FileInfo
	parent         *FileTree
	children       []*FileTree
	childrenByName map[string]*FileTree
	expanded       bool
	filter         *filter
	only           map[*FileTree]bool
}

// filter is the view filter shared by all nodes of a tree.
type filter struct {
	// Files not matching the pattern are hidden. Empty shows everything.
	pattern string
	// Directories restricted to a subset of their children by FilterGlob.
	restricted []*FileTree
}

func InitFileTree(p string) (*FileTree, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	tree := &FileTree{
		AbsPath: abs,
		info:    info,
		filter:  &filter{},
	}
	return tree, nil
}

func (t *FileTree) Name() string {
	return t.info.Name()
}

func (t *FileTree) IsDir() bool {
	if t.targetInfo != nil {
		return t.targetInfo.IsDir()
	} else {
		return t.info.IsDir()
	}
}

func (t *FileTree) Parent() *FileTree {
	return t.parent
}

func (t *FileTree) Expanded() bool {
	return t.expanded
}

func (t *FileTree) Expand() error {
	t.expanded = true
	return t.maybeLoadChildren()
}

func (t *FileTree) Collapse() {
	t.expanded = false
}

func (t *FileTree) maybeLoadChildren() error {
	if t.children != nil {
		return nil
	}
	t.children = []*FileTree{}
	t.childrenByName = map[string]*FileTree{}
	if !t.IsDir() {
		return nil
	}
	f, err := os.Open(t.AbsPath)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil
		} else {
			return err
		}
	}
	defer f.Close()
	contents, err := f.Readdir(0)
	if err != nil {
		return err
	}
	for _, content := range contents {
		childFileTree := &FileTree{
			AbsPath: filepath.Join(t.AbsPath, content.Name()),
			info:    content,
			parent:  t,
			filter:  t.filter,
		}
		if content.Mode()&os.ModeSymlink != 0 {
			if targetInfo, err := os.Stat(childFileTree.AbsPath); err == nil {
				childFileTree.targetInfo = targetInfo
				cycleFound := false
				for node := childFileTree; node.Parent() != nil; node = node.Parent() {
					if os.SameFile(node.targetInfo, targetInfo) ||
						os.SameFile(node.info, targetInfo) {
						cycleFound = true
						break
					}
				}
				if cycleFound {
					continue
				}
			} else {
				continue
			}
		}
		t.children = append(t.children, childFileTree)
		t.childrenByName[childFileTree.Name()] = childFileTree
	}
	return nil
}

func (t *FileTree) Children(order Order) ([]*FileTree, error) {
	err := t.maybeLoadChildren()
	if err != nil {
		return nil, err
	}
	children := t.children[:0:0]
	for _, child := range t.children {
		if t.shows(child) {
			children = append(children, child)
		}
	}
	if order != nil {
		sort.Slice(children, order(children))
	}
	return children, nil
}

// matchesFilter reports whether a file name passes the filter pattern. A
// pattern without wildcards matches any name containing it.
func matchesFilter(pattern string, name string) bool {
	if !HasWildcard(pattern) {
		return strings.Contains(name, pattern)
	}
	ok, _ := filepath.Match(pattern, name)
	return ok
}

func (t *FileTree) shows(child *FileTree) bool {
	if t.only != nil && !t.only[child] {
		return false
	}
	if t.filter.pattern != "" && !child.IsDir() {
		return matchesFilter(t.filter.pattern, child.Name())
	}
	return true
}

// Visible reports whether the node is not hidden by the current filter.
func (t *FileTree) Visible() bool {
	for node := t; node.parent != nil; node = node.parent {
		if !node.parent.shows(node) {
			return false
		}
	}
	return true
}

// Filter returns the pattern set by SetFilter.
func (t *FileTree) Filter() string {
	return t.filter.pattern
}

// SetFilter hides all files in the tree whose name does not match the
// pattern. Directories stay visible. It replaces any earlier filter,
// including the one set by FilterGlob. An empty pattern shows everything.
func (t *FileTree) SetFilter(pattern string) error {
	if _, err := filepath.Match(pattern, ""); err != nil {
		return fmt.Errorf("Invalid pattern %q", pattern)
	}
	for _, node := range t.filter.restricted {
		node.only = nil
	}
	t.filter.restricted = nil
	t.filter.pattern = pattern
	return nil
}

type PathNotFound struct {
	Path string
}

func (e PathNotFound) Error() string {
	return fmt.Sprint("Path not found: ", e.Path)
}

func (t *FileTree) FindPath(origPath string) (*FileTree, error) {
	var err error
	path := filepath.Clean(origPath)
	if filepath.IsAbs(path) {
		path, err = filepath.Rel(t.AbsPath, path)
		if err != nil {
			return nil, PathNotFound{origPath}
		}
	}
	parts := []string{}
	for base := filepath.Base(path); base != "."; base = filepath.Base(path) {
		path = filepath.Dir(path)
		parts = append([]string{base}, parts...)
	}
	currentNode := t
	var ok bool
	for _, part := range parts {
		err := currentNode.maybeLoadChildren()
		if err != nil {
			return nil, err
		}
        savedCurrentNode := currentNode
		currentNode, ok = currentNode.childrenByName[part]
        if !ok {
            for name, child := range savedCurrentNode.childrenByName {
                if strings.HasPrefix(name, part) {
                    currentNode = child
                    ok = true
                    break
                }
            }
        }
		if !ok {
			return nil, PathNotFound{origPath}
		}
	}
	return currentNode, nil
}

// HasWildcard reports whether the path contains glob metacharacters.
func HasWildcard(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

// FilterGlob shows only the entries matching the glob pattern and returns the
// first match in display order. It replaces any earlier filter, as SetFilter
// does. A wildcard in the last component becomes the filter pattern, which
// hides the non-matching files in the whole tree just like SetFilter, while
// directories matched against an earlier wildcard component are restricted to
// the matching children. The tree is left untouched if nothing matches.
func (t *FileTree) FilterGlob(pattern string) (*FileTree, error) {
	var err error
	path := filepath.Clean(pattern)
	if filepath.IsAbs(path) {
		path, err = filepath.Rel(t.AbsPath, path)
		if err != nil {
			return nil, PathNotFound{pattern}
		}
	}
	parts := []string{}
	for base := filepath.Base(path); base != "." && base != string(filepath.Separator); base = filepath.Base(path) {
		path = filepath.Dir(path)
		parts = append([]string{base}, parts...)
	}
	for _, part := range parts {
		if _, err := filepath.Match(part, ""); err != nil {
			return nil, fmt.Errorf("Invalid pattern %q: %w", pattern, err)
		}
	}
	m := &globMatch{only: map[*FileTree]map[*FileTree]bool{}}
	node, err := t.filterGlob(parts, m)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, PathNotFound{pattern}
	}

	filterPattern := ""
	if len(parts) > 0 && HasWildcard(parts[len(parts)-1]) {
		filterPattern = parts[len(parts)-1]
	}
	if err := t.SetFilter(filterPattern); err != nil {
		return nil, err
	}
	for dir, only := range m.only {
		dir.only = only
		t.filter.restricted = append(t.filter.restricted, dir)
	}
	for _, dir := range m.expand {
		dir.expanded = true
	}
	return node, nil
}

// globMatch collects the changes to apply once FilterGlob found a match.
type globMatch struct {
	// Children to keep in directories matched against a wildcard component
	// other than the last one.
	only map[*FileTree]map[*FileTree]bool
	// Directories containing matches, to be expanded.
	expand []*FileTree
}

func (t *FileTree) filterGlob(parts []string, m *globMatch) (*FileTree, error) {
	if len(parts) == 0 {
		return t, nil
	}
	if err := t.maybeLoadChildren(); err != nil {
		return nil, err
	}
	part := parts[0]
	if part == ".." {
		if t.parent == nil {
			return nil, nil
		}
		return t.parent.filterGlob(parts[1:], m)
	}
	if !HasWildcard(part) {
		child, ok := t.childrenByName[part]
		if !ok {
			return nil, nil
		}
		return child.filterGlob(parts[1:], m)
	}

	candidates := append(t.children[:0:0], t.children...)
	sort.Slice(candidates, ByTypeAndName(candidates))
	var first *FileTree
	kept := map[*FileTree]bool{}
	for _, child := range candidates {
		name := child.Name()
		// Like the shell, do not match hidden files unless asked for.
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(part, ".") {
			continue
		}
		if ok, _ := filepath.Match(part, name); !ok {
			continue
		}
		match, err := child.filterGlob(parts[1:], m)
		if err != nil {
			return nil, err
		}
		if match == nil {
			continue
		}
		if first == nil {
			first = match
		}
		kept[child] = true
	}
	if first == nil {
		return nil, nil
	}
	if len(parts) > 1 {
		// The last component is applied as the filter pattern instead.
		m.only[t] = kept
	}
	m.expand = append(m.expand, t)
	return first, nil
}

func (t *FileTree) Traverse(visibleOnly bool, order Order, f func(*FileTree, int) error) error {
	type treeWithDepth struct {
		tree  *FileTree
		depth int
	}
	stack := []treeWithDepth{treeWithDepth{t, 0}}
	for len(stack) > 0 {
		var current treeWithDepth
		current, stack = stack[len(stack)-1], stack[:len(stack)-1]
		err := f(current.tree, current.depth)
		if err != nil {
			return err
		}

		if !visibleOnly || current.tree.Expanded() {
			children, err := current.tree.Children(order)
			if err != nil {
				return err
			}
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, treeWithDepth{children[i], current.depth + 1})
			}
		}
	}
	return nil
}

func (t *FileTree) Prev(visibleOnly bool, order Order) (*FileTree, error) {
	if t.Parent() == nil {
		return nil, nil
	}
	siblings, err := t.Parent().Children(order)
	if err != nil {
		return nil, err
	}
	if t == siblings[0] {
		return t.Parent(), nil
	}
	var prevSibling *FileTree
	for i, sibling := range siblings {
		if sibling == t {
			prevSibling = siblings[i-1]
		}
	}
	node := prevSibling
	for {
		if !node.Expanded() && visibleOnly {
			return node, nil
		}
		children, err := node.Children(order)
		if err != nil {
			return nil, err
		}
		if len(children) == 0 {
			return node, nil
		} else {
			node = children[len(children)-1]
		}
	}
	return nil, nil
}

func (t *FileTree) Next(visibleOnly bool, order Order) (*FileTree, error) {
	if t.Expanded() || !visibleOnly {
		children, err := t.Children(order)
		if err != nil {
			return nil, err
		}
		if len(children) > 0 {
			return children[0], nil
		}
	}
	node := t
	for node.Parent() != nil {
		siblings, err := node.Parent().Children(order)
		if err != nil {
			return nil, err
		}
		for i, sibling := range siblings {
			if sibling == node && i < len(siblings)-1 {
				return siblings[i+1], nil
			}
		}
		node = node.Parent()
	}
	return nil, nil
}

type Order func([]*FileTree) func(i, j int) bool

func ByTypeAndName(children []*FileTree) func(i, j int) bool {
	return func(i, j int) bool {
		if children[i].IsDir() != children[j].IsDir() {
			return children[i].IsDir()
		} else {
			return children[i].Name() < children[j].Name()
		}
	}
}
