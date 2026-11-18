package filetree

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestName(t *testing.T) {
	tree, err := InitFileTree("testdata")
	assert.Nil(t, err)
	assert.Equal(t, "testdata", tree.Name())
}

func TestIsDir(t *testing.T) {
	tree, err := InitFileTree("testdata")
	assert.Nil(t, err)
	assert.True(t, tree.IsDir())
}

func TestChildren(t *testing.T) {
	tree, err := InitFileTree("testdata")
	assert.Nil(t, err)
	children, err := tree.Children(nil)
	assert.Nil(t, err)
	childrenNames := []string{}
	for _, childTree := range children {
		childrenNames = append(childrenNames, childTree.Name())
	}
	assert.ElementsMatch(t, []string{"dir1", "dir2", "a"}, childrenNames)
}

func TestFindPath(t *testing.T) {
	root, err := InitFileTree("testdata")
	assert.Nil(t, err)

	node, err := root.FindPath("dir1/b")
	assert.Nil(t, err)
	assert.Equal(t, "b", node.Name())

	node, err = root.FindPath(".")
	assert.Nil(t, err)
	assert.Equal(t, root, node)

	wd, err := os.Getwd()
	assert.Nil(t, err)
	node, err = root.FindPath(filepath.Join(wd, "testdata/dir1/b"))
	assert.Nil(t, err)
	assert.Equal(t, "b", node.Name())

	node, err = root.FindPath(filepath.Join(wd, "testdata"))
	assert.Nil(t, err)
	assert.Equal(t, root, node)
}

func childNames(t *testing.T, tree *FileTree) []string {
	children, err := tree.Children(ByTypeAndName)
	assert.Nil(t, err)
	names := []string{}
	for _, child := range children {
		names = append(names, child.Name())
	}
	return names
}

func TestFilterGlob(t *testing.T) {
	root, err := InitFileTree("testdata")
	assert.Nil(t, err)
	node, err := root.FilterGlob("dir1/[a-c]")
	assert.Nil(t, err)
	assert.Equal(t, "b", node.Name())
	assert.Equal(t, "[a-c]", root.Filter())
	assert.Equal(t, []string{"b"}, childNames(t, node.Parent()))
	assert.True(t, node.Parent().Expanded())
	assert.Equal(t, []string{"dir1", "dir2", "a"}, childNames(t, root))

	// Like SetFilter, the last component hides files in the whole tree.
	root, err = InitFileTree("testdata")
	assert.Nil(t, err)
	node, err = root.FilterGlob("dir1/b*")
	assert.Nil(t, err)
	assert.Equal(t, "b", node.Name())
	assert.Equal(t, "b*", root.Filter())
	assert.Equal(t, []string{"dir1", "dir2"}, childNames(t, root))
	assert.Equal(t, []string{}, childNames(t, root.childrenByName["dir2"]))

	// Wildcards in directory components restrict those directories.
	root, err = InitFileTree("testdata")
	assert.Nil(t, err)
	node, err = root.FilterGlob("dir*/c")
	assert.Nil(t, err)
	assert.Equal(t, "c", node.Name())
	assert.Equal(t, "", root.Filter())
	assert.Equal(t, []string{"dir2"}, childNames(t, root))

	// A new pattern replaces the previous one.
	node, err = root.FilterGlob("dir1/*")
	assert.Nil(t, err)
	assert.Equal(t, "b", node.Name())
	assert.Equal(t, []string{"dir1", "dir2", "a"}, childNames(t, root))

	// Nothing matches: the tree is left untouched.
	_, err = root.FilterGlob("dir1/*.pdf")
	assert.IsType(t, PathNotFound{}, err)
	assert.Equal(t, "*", root.Filter())
	assert.Equal(t, []string{"b"}, childNames(t, root.childrenByName["dir1"]))

	_, err = root.FilterGlob("dir1/[")
	assert.NotNil(t, err)
}

func TestSetFilter(t *testing.T) {
	root, err := InitFileTree("testdata")
	assert.Nil(t, err)
	assert.Nil(t, root.Expand())
	dir1 := root.childrenByName["dir1"]
	assert.Nil(t, dir1.Expand())
	b := dir1.childrenByName["b"]

	assert.Nil(t, root.SetFilter("[ac]"))
	assert.Equal(t, []string{"dir1", "dir2", "a"}, childNames(t, root))
	assert.Equal(t, []string{}, childNames(t, dir1))
	assert.False(t, b.Visible())
	assert.True(t, dir1.Visible())

	// A new filter replaces the previous one.
	assert.Nil(t, root.SetFilter("b"))
	assert.Equal(t, []string{"dir1", "dir2"}, childNames(t, root))
	assert.Equal(t, []string{"b"}, childNames(t, dir1))

	assert.NotNil(t, root.SetFilter("["))
	assert.Equal(t, "b", root.Filter())

	assert.Nil(t, root.SetFilter(""))
	assert.Equal(t, []string{"dir1", "dir2", "a"}, childNames(t, root))

	// It also replaces the restriction of FilterGlob.
	_, err = root.FilterGlob("dir*/c")
	assert.Nil(t, err)
	assert.Equal(t, []string{"dir2"}, childNames(t, root))
	assert.Nil(t, root.SetFilter("*"))
	assert.Equal(t, []string{"dir1", "dir2", "a"}, childNames(t, root))
}

func TestTraverse(t *testing.T) {
	names := []string{}
	root, err := InitFileTree("testdata")
	assert.Nil(t, err)
	err = root.Traverse(false, nil, func(node *FileTree, _ int) error {
		names = append(names, node.Name())
		return nil
	})
	assert.ElementsMatch(t, []string{"testdata", "dir1", "dir2", "a", "b", "c"}, names)
}

func TestByTypeAndName(t *testing.T) {
	nodes := []*FileTree{}
	root, err := InitFileTree("testdata")
	assert.Nil(t, err)
	err = root.Traverse(false, nil, func(node *FileTree, _ int) error {
		nodes = append(nodes, node)
		return nil
	})
	sort.Slice(nodes, ByTypeAndName(nodes))
	names := []string{}
	for _, node := range nodes {
		names = append(names, node.Name())
	}
	assert.Equal(t, []string{"dir1", "dir2", "testdata", "a", "b", "c"}, names)
}

func TestPrevNext(t *testing.T) {
	tree, err := InitFileTree("testdata")
	assert.Nil(t, err)
	names := []string{tree.Name()}
	for {
		next, err := tree.Next(false, ByTypeAndName)
		assert.Nil(t, err)
		if next == nil {
			break
		}
		tree = next
		names = append(names, tree.Name())
	}
	for {
		prev, err := tree.Prev(false, ByTypeAndName)
		assert.Nil(t, err)
		if prev == nil {
			break
		}
		tree = prev
		names = append(names, tree.Name())
	}
	assert.Equal(t, []string{"testdata", "dir1", "b", "dir2", "c", "a", "c", "dir2", "b", "dir1", "testdata"}, names)
}
