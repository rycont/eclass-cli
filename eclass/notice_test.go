package eclass

import "testing"

// 본문 안에 div가 있으면 정규식 (.*?)</div>는 첫 닫는 태그에서 잘려 나간다.
// 표나 레이아웃이 들어간 공지가 흔해서 짝을 세야 한다.
func TestInnerHTMLNested(t *testing.T) {
	page := `<td><div class="editor_content">앞<div class="x">중첩<div>더</div></div>뒤</div><div id="tbody_file"></div></td>`
	got := innerHTML(page, `<div class="editor_content">`)
	want := `앞<div class="x">중첩<div>더</div></div>뒤`
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestInnerHTMLEmptyAndMissing(t *testing.T) {
	if got := innerHTML(`<div class="editor_content"></div>`, `<div class="editor_content">`); got != "" {
		t.Fatalf("빈 본문 = %q", got)
	}
	if got := innerHTML(`<p>없음</p>`, `<div class="editor_content">`); got != "" {
		t.Fatalf("없는 태그 = %q", got)
	}
}

// 첨부 목록은 efile_list.acl이 HTML 조각으로 돌려준다.
func TestParseAttachments(t *testing.T) {
	html := `<ul class="attach_list"><li>
	  <a class="font_subtitle3 file_down" href="/ilos/co/efile_download.acl?FILE_SEQ=7YUV46SLBHOX4&amp;CONTENT_SEQ=8245962" title="첨부파일 다운로드">260901_일차공지.pdf</a>
	  <span class="font_caption1 file_size">90.9KB</span>
	</li></ul>`

	fs := parseAttachments(html)
	if len(fs) != 1 {
		t.Fatalf("첨부 %d개, want 1", len(fs))
	}
	f := fs[0]
	if f.FileName != "260901_일차공지.pdf" || f.FileSeq != "7YUV46SLBHOX4" || f.FileSize != "90.9KB" {
		t.Fatalf("첨부 = %+v", f)
	}
	// &amp;를 풀지 않으면 다운로드 URL이 깨진다
	if f.DownURL != BaseURL+"/ilos/co/efile_download.acl?FILE_SEQ=7YUV46SLBHOX4&CONTENT_SEQ=8245962" {
		t.Fatalf("URL = %q", f.DownURL)
	}
}

func TestParseAttachmentsEmpty(t *testing.T) {
	if fs := parseAttachments(`<div class="attach_container"></div>`); len(fs) != 0 {
		t.Fatalf("got %+v", fs)
	}
}
