package pool

import "strings"

// ansiFilter strips ANSI sequences that clear or swap the screen.
type ansiFilter struct {
	pending []byte
}

func (f *ansiFilter) Filter(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}

	combined := append(f.pending, data...)
	f.pending = nil

	out := make([]byte, 0, len(combined))
	for i := 0; i < len(combined); {
		if combined[i] != 0x1b {
			out = append(out, combined[i])
			i++
			continue
		}

		if i+1 >= len(combined) {
			f.pending = append(f.pending, combined[i:]...)
			break
		}

		next := combined[i+1]
		if next == '[' {
			end := i + 2
			for end < len(combined) && (combined[end] < 0x40 || combined[end] > 0x7E) {
				end++
			}
			if end >= len(combined) {
				f.pending = append(f.pending, combined[i:]...)
				break
			}

			seq := combined[i : end+1]
			if shouldStripCSI(seq) {
				i = end + 1
				continue
			}

			out = append(out, seq...)
			i = end + 1
			continue
		}

		if next == 'c' {
			i += 2
			continue
		}

		out = append(out, combined[i])
		i++
	}

	return out
}

func shouldStripCSI(seq []byte) bool {
	if len(seq) < 3 {
		return false
	}

	params := string(seq[2 : len(seq)-1])
	final := seq[len(seq)-1]
	switch final {
	case 'J':
		return params == "2" || params == "3"
	case 'h', 'l':
		return strings.HasPrefix(params, "?1049") || strings.HasPrefix(params, "?1047") || strings.HasPrefix(params, "?47")
	default:
		return false
	}
}
