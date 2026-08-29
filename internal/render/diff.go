package render

import "strings"

// diffContext is how many unchanged lines are shown around each change.
const diffContext = 3

type diffOp struct {
	sign byte // ' ', '-' or '+'
	text string
}

// Diff returns a line diff of a and b: unchanged lines prefixed with a space,
// removals with "-", additions with "+", and long unchanged runs elided as
// "...". Identical inputs give the empty string.
func Diff(a, b string) string {
	if a == b {
		return ""
	}
	ops := diffOps(splitDiffLines(a), splitDiffLines(b))

	keep := make([]bool, len(ops))
	for i, op := range ops {
		if op.sign == ' ' {
			continue
		}
		for k := max(0, i-diffContext); k <= min(len(ops)-1, i+diffContext); k++ {
			keep[k] = true
		}
	}

	var out strings.Builder
	elided := false
	for i, op := range ops {
		if !keep[i] {
			elided = true
			continue
		}
		if elided {
			out.WriteString("...\n")
			elided = false
		}
		out.WriteByte(op.sign)
		out.WriteString(op.text)
		out.WriteByte('\n')
	}
	if elided {
		out.WriteString("...\n")
	}
	return out.String()
}

// diffOps walks a longest-common-subsequence table over the two line slices.
// Quadratic, which is fine for documents of this size.
func diffOps(x, y []string) []diffOp {
	n, m := len(x), len(y)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if x[i] == y[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}

	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case x[i] == y[j]:
			ops = append(ops, diffOp{' ', x[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, diffOp{'-', x[i]})
			i++
		default:
			ops = append(ops, diffOp{'+', y[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, diffOp{'-', x[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, diffOp{'+', y[j]})
	}
	return ops
}

func splitDiffLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}
