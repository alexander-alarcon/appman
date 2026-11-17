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
	Image        Image
	Type         AppType
	Offset       int64
	Metadata     Metadata
	DesktopFile  string
	IconFile     string
	IconExtract  string
	Origins      map[string]string
	Warnings     []string
	Degraded     bool
	ExtractTool  string
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
	defer f.Close()

	// detect ELF/type2
	ef, err := elf.NewFile(f)
	if err == nil {
		defer ef.Close()
		// AppImage type 2 is typically ELF + appended squashfs
		off := elfEndOffset(ef)
		info.Offset = off
		if off > 0 && off < info.Image.Size {
			if checkMagicAt(f, off) {
				info.Type = Type2
				info.Origins["type"] = "elf+squashfs-magic"
				return
			}
		}
		// scan for hsqs magic as fallback
		if off2 := scanSquashfsMagic(f, info.Image.Size); off2 > 0 {
			info.Offset = off2
			info.Type = Type2
			info.Origins["type"] = "squashfs-magic-scan"
			return
		}
		info.Type = Type1
		info.Origins["type"] = "elf-no-squashfs"
		return
	}

	// non-ELF: scan for magic
	if off2 := scanSquashfsMagic(f, info.Image.Size); off2 > 0 {
		info.Offset = off2
		info.Type = Type2
		info.Origins["type"] = "squashfs-magic-scan"
		return
	}
	info.Type = TypeUnknown
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

func checkMagicAt(r io.ReaderAt, off int64) bool {
	b := make([]byte, 4)
	n, err := r.ReadAt(b, off)
	return err == nil && n == 4 && bytes.Equal(b, squashfsMagic)
}

func scanSquashfsMagic(r io.ReaderAt, size int64) int64 {
	// scan in 4-byte steps up to reasonable range; AppImages can be large, so sample/scan chunks
	buf := make([]byte, 4096)
	for off := int64(0); off < size; off += int64(len(buf) - 3) {
		n, err := r.ReadAt(buf, off)
		if n < 4 {
			if err != nil {
				break
			}
			continue
		}
		chunk := buf[:n]
		idx := bytes.Index(chunk, squashfsMagic)
		if idx >= 0 {
			cand := off + int64(idx)
			// squashfs superblock starts at magic
			return cand
		}
		if err != nil {
			break
		}
	}
	return 0
}

func extractMetadata(info *InspectInfo) error {
	tool, err := exec.LookPath("unsquashfs")
	if err != nil {
		return fmt.Errorf("unsquashfs not found (install squashfs-tools) - cannot extract embedded metadata")
	}
	info.ExtractTool = tool

	files, err := listSquashfs(tool, info.Image.Path, info.Offset)
	if err != nil {
		return fmt.Errorf("list squashfs: %w", err)
	}
	desktop := pickDesktop(files)
	if desktop == "" {
		info.Warnings = append(info.Warnings, "no .desktop file found inside AppImage")
	} else {
		info.DesktopFile = desktop
		data, err := catSquashfs(tool, info.Image.Path, info.Offset, desktop)
		if err != nil {
			info.Warnings = append(info.Warnings, fmt.Sprintf("extract desktop: %v", err))
		} else {
			info.Metadata, info.Origins = parseDesktopEntry(data)
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

func listSquashfs(tool, image string, offset int64) ([]string, error) {
	cmd := exec.Command(tool, "-o", fmt.Sprintf("%d", offset), "-ll", image)
	out, err := cmd.Output()
	if err != nil {
		// try without -ll details
		cmd = exec.Command(tool, "-o", fmt.Sprintf("%d", offset), "-l", image)
		out, err = cmd.Output()
		if err != nil {
			return nil, err
		}
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
		// common fallbacks
		for _, name := range []string{".DirIcon", "AppIcon.png", "appicon.png"} {
			for _, f := range files {
				if filepath.Base(f) == name || f == name {
					return f
				}
			}
		}
		return ""
	}
	base := iconName
	if filepath.Ext(base) != "" {
		// exact/endswith match
		for _, f := range files {
			if filepath.Base(f) == base {
				return f
			}
		}
	}
	// try common extensions and locations
	exts := []string{".png", ".svg", ".xpm"}
	for _, f := range files {
		b := filepath.Base(f)
		bno := strings.TrimSuffix(b, filepath.Ext(b))
		if bno == base || b == base {
			for _, e := range exts {
				if strings.EqualFold(filepath.Ext(b), e) || filepath.Ext(b) == "" {
					return f
				}
			}
			return f
		}
	}
	for _, e := range exts {
		want := base + e
		for _, f := range files {
			if filepath.Base(f) == want {
				return f
			}
		}
	}
	// prefer icons under usr/share/icons or pixmaps
	for _, f := range files {
		bno := strings.TrimSuffix(filepath.Base(f), filepath.Ext(filepath.Base(f)))
		if bno == base && (strings.Contains(f, "icons") || strings.Contains(f, "pixmaps")) {
			return f
		}
	}
	return ""
}

func catSquashfs(tool, image string, offset int64, file string) ([]byte, error) {
	cmd := exec.Command(tool, "-o", fmt.Sprintf("%d", offset), "-cat", image, file)
	return cmd.Output()
}

func extractFile(tool, image string, offset int64, file string) (string, error) {
	tmp, err := os.MkdirTemp("", "appman-icon-*")
	if err != nil {
		return "", err
	}
	// extract single file to temp dir
	cmd := exec.Command(tool, "-o", fmt.Sprintf("%d", offset), "-d", tmp, "-f", image, file)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%v: %s", err, string(out))
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
