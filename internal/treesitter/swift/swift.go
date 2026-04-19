package swift

// #include "parser.h"
// const TSLanguage *tree_sitter_swift(void);
import "C"
import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// GetLanguage returns the swift language definition for tree-sitter
func GetLanguage() *tree_sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_swift())
	return tree_sitter.NewLanguage(ptr)
}
