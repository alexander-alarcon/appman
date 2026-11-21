package appimage

import (
	"bufio"
	"bytes"
	"debug/elf"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type AppType string

const (
	TypeUnknown AppType = "unknown"
	Type1       AppType = "type1"
	Type2       AppType = "type2"
)

type Metadata struct {
	Name       string
	Version    string
	Icon       string
	Categories string
	Comment    string
	DesktopID  string
}

type InspectInfo struct {
	Image       Image
	Type        AppType
	Offset      int64
	Metadata    Metadata
	DesktopFile string
	IconFile    string
	IconExtract string
	Origins     map[string]string
	Warnings    []string
	Degraded    bool
	ExtractTool string
}

var squashfsMagic = []byte{'h', 's', 'q', 's'}

func Inspect(path string) (*InspectInfo, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	info := &InspectInfo{
		Image: Image{
			Path:      path,
			Filename:  filepath.Base(path),
			NameGuess: guessName(filepath.Base(path)),
			Size:      fi.Size(),
			ModTime:   fi.ModTime(),
			Mode:      fi.Mode(),
		},
		Type:    TypeUnknown,
		Origins: map[string]string{},
	}
	detectTypeAndOffset(info)
	if info.Type != Type2 {
		info.Warnings = append(info.Warnings, "unsupported or unknown AppImage type; metadata extraction limited")
		info.Degraded = true
		applyFallbacks(info)
		return info, nil
	}
	if err := extractMetadata(info); err != nil {
		info.Warnings = append(info.Warnings, err.Error())
		info.Degraded = true
	}
	applyFallbacks(info)
	return info, nil
}

func detectTypeAndOffset(info *InspectInfo) {
	f, err := os.Open(info.Image.Path)
	if err != nil {
		info.Warnings = append(info.Warnings, fmt.Sprintf("open: %v", err))
		return
	}
	defer func() { _ = f.Close() }()
	ef, err := elf.NewFile(f)
	if err == nil {
		defer func() { _ = ef.Close() }()
		// collect candidates: elf-derived end and all hsqs occurrences
		off := elfEndOffset(ef)
		magics := scanSquashfsMagics(f, info.Image.Size)
		cands := uniqueOffsets(append([]int64{off}, magics...))
		if len(cands) > 0 {
			info.Type = Type2
			info.Origins["type"] = "elf+squashfs-candidates"
			// provisional; will be validated in extractMetadata
			info.Offset = pickBestCandidateOffset(cands, off)
			info.Origins["offset_candidates"] = fmt.Sprintf("%v", cands)
			return
		}
		info.Type = Type1
		info.Origins["type"] = "elf-no-squashfs"
		return
	}

	magics := scanSquashfsMagics(f, info.Image.Size)
	if len(magics) > 0 {
		info.Type = Type2
		info.Offset = magics[0]
		info.Origins["type"] = "squashfs-magic-scan"
		info.Origins["offset_candidates"] = fmt.Sprintf("%v", magics)
		return
	}
	info.Type = TypeUnknown
}

func uniqueOffsets(in []int64) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, v := range in {
		if v <= 0 {
			continue
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func pickBestCandidateOffset(cands []int64, preferred int64) int64 {
	for _, c := range cands {
		if c == preferred {
			return c
		}
	}
	if len(cands) > 0 {
		return cands[len(cands)-1]
	}
	return 0
}

func elfEndOffset(ef *elf.File) int64 {
	var maxEnd int64
	for _, p := range ef.Progs {
		end := int64(p.Off + p.Filesz)
		if end > maxEnd {
			maxEnd = end
		}
	}
	for _, s := range ef.Sections {
		if s.Type == elf.SHT_NOBITS {
			continue
		}
		end := int64(s.Offset + s.Size)
		if end > maxEnd {
			maxEnd = end
		}
	}
	// align up a bit; squashfs commonly follows
	return maxEnd
}

func scanSquashfsMagics(r io.ReaderAt, size int64) []int64 {
	buf := make([]byte, 4096)
	var found []int64
	seen := map[int64]bool{}
	for off := int64(0); off < size; off += int64(len(buf) - 3) {
		n, err := r.ReadAt(buf, off)
		if n < 4 {
			if err != nil {
				break
			}
			continue
		}
		chunk := buf[:n]
		search := 0
		for {
			idx := bytes.Index(chunk[search:], squashfsMagic)
			if idx < 0 {
				break
			}
			cand := off + int64(search+idx)
			if !seen[cand] {
				seen[cand] = true
				found = append(found, cand)
			}
			search += idx + 1
		}
		if err != nil {
			break
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i] < found[j] })
	return found
}

func extractMetadata(info *InspectInfo) error {
	tool, err := exec.LookPath("unsquashfs")
	if err != nil {
		return fmt.Errorf("unsquashfs not found (install squashfs-tools) - cannot extract embedded metadata")
	}
	info.ExtractTool = tool

	offsets := candidateOffsets(info)
	files, usedOffset, err := listSquashfsWithCandidates(tool, info.Image.Path, offsets)
	if err != nil {
		return fmt.Errorf("list squashfs: %w", err)
	}
	info.Offset = usedOffset
	info.Origins["offset"] = fmt.Sprintf("%d", usedOffset)

	desktop := pickDesktop(files)
	if desktop == "" {
		info.Warnings = append(info.Warnings, "no .desktop file found inside AppImage")
	} else {
		info.DesktopFile = desktop
		data, err := catSquashfs(tool, info.Image.Path, info.Offset, desktop)
		if err != nil {
			info.Warnings = append(info.Warnings, fmt.Sprintf("extract desktop: %v", err))
		} else {
			md, origins := parseDesktopEntry(data)
			info.Metadata = md
			for k, v := range origins {
				info.Origins[k] = v
			}
			info.Origins["desktop_file"] = "embedded:" + desktop
		}
	}

	iconPath := pickIcon(files, info.Metadata.Icon)
	if iconPath != "" {
		info.IconFile = iconPath
		extracted, err := extractFile(tool, info.Image.Path, info.Offset, iconPath)
		if err != nil {
			info.Warnings = append(info.Warnings, fmt.Sprintf("extract icon: %v", err))
		} else {
			info.IconExtract = extracted
			info.Origins["icon_file"] = "embedded:" + iconPath
		}
	} else if info.Metadata.Icon != "" {
		info.Warnings = append(info.Warnings, fmt.Sprintf("icon %q referenced but not found in AppImage", info.Metadata.Icon))
	}

	return nil
}

func candidateOffsets(info *InspectInfo) []int64 {
	// parse stored candidates plus current/probed offset
	var cands []int64
	if s, ok := info.Origins["offset_candidates"]; ok {
		s = strings.Trim(s, "[]")
		for _, p := range strings.Fields(s) {
			p = strings.TrimSpace(p)
			var v int64
			if _, err := fmt.Sscan(p, &v); err == nil && v > 0 {
				cands = append(cands, v)
			}
		}
	}
	if info.Offset > 0 {
		cands = append(cands, info.Offset)
	}
	// as a fallback, rescan magics
	f, err := os.Open(info.Image.Path)
	if err == nil {
		cands = append(cands, scanSquashfsMagics(f, info.Image.Size)...)
		_ = f.Close()
	}
	return uniqueOffsets(cands)
}

func listSquashfsWithCandidates(tool, image string, offsets []int64) ([]string, int64, error) {
	if len(offsets) == 0 {
		offsets = []int64{0}
	}
	var lastErr error
	// try in reverse order too? many AppImages have real squashfs later; try all and prefer one that yields files
	for _, off := range offsets {
		files, err := listSquashfs(tool, image, off)
		if err == nil && len(files) > 0 {
			return files, off, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("empty listing at offset %d", off)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no valid squashfs offset found")
	}
	return nil, 0, lastErr
}

func listSquashfs(tool, image string, offset int64) ([]string, error) {
	cmd := exec.Command(tool, "-o", fmt.Sprintf("%d", offset), "-ll", image)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// try without -ll details
		cmd = exec.Command(tool, "-o", fmt.Sprintf("%d", offset), "-l", image)
		out2, err2 := cmd.CombinedOutput()
		if err2 != nil {
			msg := strings.TrimSpace(string(out2))
			if msg == "" {
				msg = strings.TrimSpace(string(out))
			}
			if msg != "" {
				return nil, fmt.Errorf("%w: %s", err2, msg)
			}
			return nil, err2
		}
		out = out2
	}
	var files []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// -ll: permissions links user group size date time path [-> target]
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		p := fields[len(fields)-1]
		if strings.Contains(line, " -> ") {
			// skip symlink target handling complexity; keep name
			for i, f := range fields {
				if f == "->" && i > 0 {
					p = fields[i-1]
					break
				}
			}
		}
		// for -ll path is usually last or before ->
		if len(fields) >= 6 && strings.HasPrefix(fields[0], "-") || strings.HasPrefix(fields[0], "l") || strings.HasPrefix(fields[0], "d") {
			// try to reconstruct path (may contain spaces) after time field (index 5: mmm dd hh:mm or mmm dd yyyy)
			if len(fields) >= 7 {
				// heuristic: path starts at field 5 or 6 depending format
				cand := strings.Join(fields[5:], " ")
				if strings.Contains(cand, " -> ") {
					cand = strings.SplitN(cand, " -> ", 2)[0]
				}
				if strings.HasPrefix(cand, "squashfs-root") || strings.HasPrefix(cand, "/") || strings.Contains(cand, "/") || strings.HasSuffix(cand, ".desktop") || strings.Contains(cand, ".") {
					p = cand
				}
			}
		}
		p = strings.TrimPrefix(p, "squashfs-root/")
		p = strings.TrimPrefix(p, "./")
		if p != "" {
			files = append(files, p)
		}
	}
	return files, sc.Err()
}

func pickDesktop(files []string) string {
	var cands []string
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ".desktop") {
			cands = append(cands, f)
		}
	}
	if len(cands) == 0 {
		return ""
	}
	sort.Slice(cands, func(i, j int) bool {
		di := strings.Count(cands[i], "/")
		dj := strings.Count(cands[j], "/")
		if di != dj {
			return di < dj
		}
		return cands[i] < cands[j]
	})
	return cands[0]
}

func pickIcon(files []string, iconName string) string {
	if iconName == "" {
		return pickFallbackIcon(files)
	}
	baseName := filepath.Base(iconName)
	knownExts := []string{".png", ".svg", ".svgz", ".xpm", ".ico"}
	lowerBase := strings.ToLower(baseName)
	hasImageExt := false
	for _, e := range knownExts {
		if strings.HasSuffix(lowerBase, e) {
			hasImageExt = true
			break
		}
	}

	// if a concrete image filename is referenced, try exact basename match
	if hasImageExt {
		for _, f := range files {
			if strings.EqualFold(filepath.Base(f), baseName) {
				return f
			}
		}
	}

	nameNoExt := baseName
	if hasImageExt {
		nameNoExt = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	}

	// match by icon id/name (handles reverse-DNS names with dots, e.g. org.shotcut.Shotcut)
	var matches []string
	for _, f := range files {
		b := filepath.Base(f)
		bno := strings.TrimSuffix(b, filepath.Ext(b))
		lb := strings.ToLower(b)
		if strings.EqualFold(b, baseName) || strings.EqualFold(bno, nameNoExt) || strings.EqualFold(bno, baseName) {
			// prefer image-like files
			if isImageFile(lb) || b == ".DirIcon" {
				matches = append(matches, f)
			} else if !hasImageExt {
				matches = append(matches, f)
			}
		}
	}
	if len(matches) > 0 {
		sort.Slice(matches, func(i, j int) bool { return iconScore(matches[i]) > iconScore(matches[j]) })
		return matches[0]
	}

	// try name + known extensions
	for _, e := range knownExts {
		want := nameNoExt + e
		for _, f := range files {
			if strings.EqualFold(filepath.Base(f), want) {
				return f
			}
		}
	}

	// fallback to canonical AppImage icon
	if fb := pickFallbackIcon(files); fb != "" {
		return fb
	}
	return ""
}

func pickFallbackIcon(files []string) string {
	for _, name := range []string{".DirIcon", "AppIcon.png", "appicon.png", ".DirIcon.png"} {
		for _, f := range files {
			if filepath.Base(f) == name || f == name {
				return f
			}
		}
	}
	return ""
}

func isImageFile(lowerBase string) bool {
	for _, e := range []string{".png", ".svg", ".svgz", ".xpm", ".ico", ".jpg", ".jpeg"} {
		if strings.HasSuffix(lowerBase, e) {
			return true
		}
	}
	return false
}

func iconScore(p string) int {
	s := 0
	lp := strings.ToLower(p)
	if strings.Contains(lp, "256") {
		s += 3
	}
	if strings.Contains(lp, "128") {
		s += 2
	}
	if strings.Contains(lp, "icons") || strings.Contains(lp, "pixmaps") {
		s += 2
	}
	if strings.HasPrefix(p, ".DirIcon") || filepath.Base(p) == ".DirIcon" {
		s += 1
	}
	if strings.HasSuffix(strings.ToLower(p), ".png") {
		s += 1
	}
	return s
}

func catSquashfs(tool, image string, offset int64, file string) ([]byte, error) {
	candidates := []string{file, strings.TrimPrefix(file, "squashfs-root/"), "squashfs-root/" + strings.TrimPrefix(file, "squashfs-root/")}
	seen := map[string]bool{}
	var lastErr error
	for _, c := range candidates {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		cmd := exec.Command(tool, "-o", fmt.Sprintf("%d", offset), "-cat", image, c)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return out, nil
		}
		lastErr = fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil, lastErr
}

func extractFile(tool, image string, offset int64, file string) (string, error) {
	tmp, err := os.MkdirTemp("", "appman-icon-*")
	if err != nil {
		return "", err
	}
	// extract single file to temp dir
	cmd := exec.Command(tool, "-o", fmt.Sprintf("%d", offset), "-d", tmp, "-f", image, file)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	outPath := filepath.Join(tmp, filepath.FromSlash(file))
	if _, err := os.Stat(outPath); err != nil {
		// sometimes extracts under basename
		alt := filepath.Join(tmp, filepath.Base(file))
		if _, err2 := os.Stat(alt); err2 == nil {
			return alt, nil
		}
		return "", err
	}
	return outPath, nil
}

func parseDesktopEntry(data []byte) (Metadata, map[string]string) {
	md := Metadata{}
	orig := map[string]string{}
	inDesktop := false
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sec := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			inDesktop = (sec == "Desktop Entry")
			continue
		}
		if !inDesktop {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch k {
		case "Name":
			if md.Name == "" {
				md.Name = v
				orig["name"] = "desktop"
			}
		case "Version":
			if md.Version == "" {
				md.Version = v
				orig["version"] = "desktop"
			}
		case "Icon":
			if md.Icon == "" {
				md.Icon = v
				orig["icon"] = "desktop"
			}
		case "Categories":
			if md.Categories == "" {
				md.Categories = v
				orig["categories"] = "desktop"
			}
		case "Comment":
			if md.Comment == "" {
				md.Comment = v
				orig["comment"] = "desktop"
			}
		case "X-AppImage-Version":
			if md.Version == "" {
				md.Version = v
				orig["version"] = "desktop:X-AppImage-Version"
			}
		}
	}
	return md, orig
}

func applyFallbacks(info *InspectInfo) {
	if info.Metadata.Name == "" {
		info.Metadata.Name = info.Image.NameGuess
		info.Origins["name"] = "filename"
	}
	if info.Metadata.Version == "" {
		// try to infer from filename
		base := strings.TrimSuffix(info.Image.Filename, filepath.Ext(info.Image.Filename))
		parts := strings.Split(base, "-")
		if len(parts) > 1 {
			last := parts[len(parts)-1]
			if len(last) > 0 && last[0] >= '0' && last[0] <= '9' {
				info.Metadata.Version = last
				info.Origins["version"] = "filename"
			}
		}
	}
}

func (i *InspectInfo) DisplayName() string {
	if i.Metadata.Name != "" {
		return i.Metadata.Name
	}
	return i.Image.NameGuess
}
