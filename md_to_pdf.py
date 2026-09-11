import sys
import os
#from markdown_pdf import MarkdownPdf, Section
from reportlab.lib.pagesizes import A4
from reportlab.pdfgen import canvas
from reportlab.lib import colors
from reportlab.lib.units import inch
from reportlab.pdfbase import pdfmetrics
from reportlab.lib.utils import ImageReader
from reportlab.pdfbase.ttfonts import TTFont
from PyPDF2 import PdfMerger

from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle, PageBreak
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.pdfbase import pdfmetrics
import markdown2


def create_cover_page(subtitle, summary, timeline, clustername, output_path,bg_image,logo_image):
    """Create a professional-looking cover page."""

    # Register better fonts
    pdfmetrics.registerFont(TTFont("Montserrat-Bold",
                                   "/usr/share/fonts/truetype/montserrat/Montserrat-Bold.ttf"))
    pdfmetrics.registerFont(TTFont("Lato-Regular",
                                   "/usr/share/fonts/truetype/lato/Lato-Regular.ttf"))
    pdfmetrics.registerFont(TTFont("Lato-Semibold",
                                   "/usr/share/fonts/truetype/lato/Lato-Semibold.ttf"))
    pdfmetrics.registerFont(TTFont("SourceSans-Regular",
                                   "/usr/share/fonts/truetype/source-sans-pro/SourceSansPro-Regular.ttf"))

    c = canvas.Canvas(output_path, pagesize=A4)
    width, height = A4

    # 1. full-page background
    if os.path.exists(bg_image):
        c.drawImage(bg_image, 0, 0, width=width, height=height)

    # 2. Company Logo – Top-Left
    if os.path.exists(logo_image):
        c.drawImage(logo_image, 0.6*inch, height - 1.75*inch,
                    width=1.6*inch, preserveAspectRatio=True, mask='auto')

    # 3. Cluster Name (small font)
    c.setFont("Lato-Regular", 15)
    c.setFillColorRGB(0.82, 0.82, 0.82)
    c.drawString(0.6*inch, height - 2.15*inch, clustername)

    # 4. Title (large font, bold)
    c.setFont("Montserrat-Bold", 32)
    c.setFillColorRGB(1, 1, 1)
    c.drawString(0.6*inch, height - 2.9*inch, "Kubernetes Usage Report")

    # 5. Month & Date (medium font)
    c.setFont("Lato-Semibold", 22)
    c.setFillColorRGB(1, 1, 1)
    c.drawString(0.6*inch, height - 3.35*inch, "for ")

    # Measure width of "for "
    for_width=c.stringWidth("for ","Lato-Semibold",22)

    # Draw "Nov 2025" in neon blue
    c.setFillColorRGB(0.55, 0.78, 1)   # neon accent
    c.drawString(0.6*inch + for_width, height - 3.35*inch, subtitle)   # subtitle = "Nov 2025"

    # 6. Summary paragraph
    summary_style = ParagraphStyle(
    'CoverSummary',
    fontName="SourceSans-Regular",
    fontSize=14,
    leading=18,
    textColor=colors.HexColor("#EBEBEB"),
    ) 

    summary_paragraph = Paragraph(summary, summary_style)
    summary_paragraph.wrapOn(c, width - 1.2*inch, height)     # width, available height
    summary_paragraph.drawOn(c, 0.6*inch, height - 4.1*inch)

    # 7. Analysis timeline at bottom
    c.setFont("Lato-Semibold", 12)
    c.setFillColorRGB(0.55, 0.78, 1)
    c.drawRightString(width - 0.6*inch, 0.85*inch, timeline)

    c.showPage()
    c.save()

def markdown_to_pdf(input_path, output_path):
    from bs4 import BeautifulSoup
    from reportlab.platypus import Spacer, Paragraph, Table, TableStyle,ListFlowable, ListItem
    import re

    # Register emoji-capable font
    pdfmetrics.registerFont(TTFont("Symbola", "/usr/share/fonts/truetype/ancient-scripts/Symbola_hint.ttf"))

    styles = getSampleStyleSheet()
    styles.add(ParagraphStyle("Body", fontName="Symbola", fontSize=11, leading=15))
    styles.add(ParagraphStyle("Heading", fontName="Montserrat-Bold", fontSize=12,leading=24,spaceBefore=12, spaceAfter=18))
    styles.add(ParagraphStyle("ListItem", fontName="Symbola", fontSize=11,leading=16, leftIndent=12,spaceBefore=4,spaceAfter=4))
    styles.add(ParagraphStyle("ListItemIndented", fontName="Symbola", fontSize=11, leading=16, leftIndent=24,spaceBefore=3,spaceAfter=3))

    # Read markdown → HTML
    md = open(input_path, "r", encoding="utf-8").read()
    html = markdown2.markdown(md, extras=["tables"])
    soup = BeautifulSoup(html, "html.parser")

    story = []

    def sanitize_html_fragment(s):
        s = re.sub(r"</br\s*>", "", s)
        s = re.sub(r"<br\s*/?>", "<br/>", s)
        s = re.sub(r"</br>", "",s)
        # replace literal bullets if any
        s = s.replace("•", "&bull;")
        return s

    # helper: recursively build ListFlowable from <ul>/<ol> element
    def process_list(list_element, ordered=False, level=0):
        items = []
        for li in list_element.find_all("li", recursive=False):
            # li may contain nested lists; extract text/html of top-level contents excluding nested <ul>/<ol>
            # Build inner HTML for the li's textual part
            li_clone = BeautifulSoup(str(li), "html.parser")
            for nested in li_clone.find_all(["ul", "ol"]):
                nested.extract()  # remove nested lists for the main paragraph
            li_html = sanitize_html_fragment(str(li_clone))
            # Trim outer <li> tags if present (Paragraph can parse HTML)
            # Use a paragraph as content for list item
            para = Paragraph(li_html, styles["ListItem"] if level == 0 else styles["ListItemIndented"])

            # If li has nested lists, process them recursively
            nested_lists = li.find_all(["ul", "ol"], recursive=False)
            if nested_lists:
                # For simple support, only consider the first nested list at this li (common case)
                nested = nested_lists[0]
                nested_flow = process_list(nested, ordered=(nested.name == "ol"), level=level+1)
                # create a ListItem with the paragraph and nested list after it
                items.append(ListItem([para, nested_flow], leftIndent=6,spaceBefore=4,spaceAfter=4))
            else:
                items.append(ListItem(para, leftIndent=6,spaceBefore=4,spaceAfter=4))

        # Build ListFlowable: ordered -> bulletType '1', else bulletType 'bullet'
        if ordered:
            lf = ListFlowable(items, bulletType='1', start='1', leftIndent=18+12*(level), bulletFontName="Symbola",spaceBefore=4,spaceAfter=4,bulletIndent=0)
        else:
            lf = ListFlowable(items, bulletType='bullet', leftIndent=18+12*(level), bulletFontName="Symbola",bulletIndent=0,spaceBefore=4,spaceAfter=4)
        return lf

    # Walk entire document in order
    for element in soup.contents:

        # ---------------------------
        # TRUE MARKDOWN HEADINGS (h1/h2/h3)
        # ---------------------------
        if element.name in ["h1", "h2", "h3","h4"]:
            raw = element.get_text(strip=True)
            raw = sanitize_html_fragment(raw)

            story.append(Paragraph(f"<b>{raw}</b>", styles["Heading"]))
            story.append(Spacer(1, 8))
            continue

        # ---------------------------
        # AI-GENERATED HEADINGS: <p><strong>Heading</strong></p>
        # ---------------------------
        if element.name == "p":
            strong_tags = element.find_all("strong")

            if len(strong_tags) == 1:
                strong_text = strong_tags[0].get_text(strip=True)
                full_text = element.get_text(strip=True)

                # Strong covers entire paragraph AND is short → treat as heading
                if strong_text == full_text and len(full_text) < 120:
                    clean = sanitize_html_fragment(full_text)
                    story.append(Paragraph(f"<b>{clean}</b>", styles["Heading"]))
                    story.append(Spacer(1, 8))
                    continue

        # Lists (unordered / ordered)
        if element.name in ["ul", "ol"]:
            ordered = (element.name == "ol")
            lf = process_list(element, ordered=ordered, level=0)
            story.append(lf)
            story.append(Spacer(1, 8))
            continue

        # ---------------------------------------------------------
        # PARAGRAPHS (Normal text)
        # ---------------------------------------------------------
        if element.name == "p":
            txt = sanitize_html_fragment(str(element))

            story.append(Paragraph(txt, styles["Body"]))
            continue

        # ---------------------------
        # TABLES
        # ---------------------------
        if element.name == "table":
            rows = []
            for tr in element.find_all("tr"):
                cells = []
                for td in tr.find_all(["td", "th"]):
                    cell = sanitize_html_fragment(str(td))
                    cells.append(Paragraph(cell, styles["Body"]))
                rows.append(cells)

            table = Table(rows, repeatRows=1)
            table.setStyle(TableStyle([
                # Full grid
                ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#444444")),

                # Header background
                ('BACKGROUND', (0,0), (-1,0), colors.HexColor("#D9E6F2")),

                # Header text
                ('TEXTCOLOR', (0,0), (-1,0), colors.HexColor("#1A1A1A")),
                ('FONTNAME', (0,0), (-1,0), "Montserrat-Bold"),
                ('FONTSIZE', (0,0), (-1,0), 11),

                # Header padding
                ('TOPPADDING', (0,0), (-1,0), 6),
                ('BOTTOMPADDING', (0,0), (-1,0), 6),
                ('LEFTPADDING', (0,0), (-1,0), 6),
                ('RIGHTPADDING', (0,0), (-1,0), 6),

                # Body font
                ('FONTNAME', (0,1), (-1,-1), "Symbola"),
                ('TEXTCOLOR', (0,1), (-1,-1), colors.black),
                ('FONTSIZE', (0,1), (-1,-1), 10),

                # Body padding
                ('TOPPADDING', (0,1), (-1,-1), 4),
                ('BOTTOMPADDING', (0,1), (-1,-1), 4),

                # Center alignment for all
                ('ALIGN', (0,0), (-1,-1), 'CENTER'),
                ('VALIGN', (0,0), (-1,-1), 'MIDDLE'),
            ]))

            story.append(table)
            story.append(Spacer(1, 12))
            continue

        # ---------------------------
        # HR / SEPARATORS
        # ---------------------------
        if element.name == "hr":
            story.append(Spacer(1, 10))
            continue

    # Final PDF
    SimpleDocTemplate(output_path, pagesize=A4).build(story)

def image_to_pdf(image_path, output_path):
    """Convert a single image to a one-page PDF."""
    c = canvas.Canvas(output_path, pagesize=A4)
    width, height = A4
    if os.path.exists(image_path):
        # Scale image to fit within A4
        c.drawImage(image_path, 30, 100, width - 60, height - 200, preserveAspectRatio=True, anchor='c')
        c.showPage()
    c.save()

def merge_pdfs(pdf_paths, output_path):
    """Merge multiple PDFs into a single PDF."""
    merger = PdfMerger()
    for path in pdf_paths:
        if os.path.exists(path):
            merger.append(path)
    merger.write(output_path)
    merger.close()
    print(f"✅ Final merged PDF: {output_path}")

def main():
    if len(sys.argv) < 6:
        print("Usage: md_to_pdf.py <input_markdown> <output_pdf>")
        sys.exit(1)

    md_path = sys.argv[1]
    final_pdf = sys.argv[2]
    subtitle=sys.argv[3]
    summary=sys.argv[4]
    timeline=sys.argv[5]
    clustername=sys.argv[6]

    base_dir = "/etc/reports/cluster-usage-report"
    cover_pdf = os.path.join(base_dir, "cover_page.pdf")
    temp_pdf = os.path.join(base_dir, "temp_content.pdf")
    metrics_img = os.path.join(base_dir, "metrics_graph.png")
    bg_image= os.path.join("/app","bg_image.png")
    logo_image=os.path.join("/app","logo_image.png")

    # Step 1: Cover Page
    create_cover_page(subtitle, summary, timeline,clustername, cover_pdf,bg_image,logo_image)

    # Step 2: Markdown → PDF
    markdown_to_pdf(md_path, temp_pdf)

    # Step 3: Convert graph image to PDFs
    metrics_pdf = os.path.join(base_dir, "metrics_page.pdf")

    if os.path.exists(metrics_img):
        image_to_pdf(metrics_img, metrics_pdf)

    # Step 4: Merge all parts into final report
    pdf_parts = [cover_pdf,temp_pdf, metrics_pdf]
    merge_pdfs(pdf_parts, final_pdf)

    # Step 5: Cleanup temp files
    for f in [cover_pdf,temp_pdf, metrics_pdf]:
        if os.path.exists(f):
            os.remove(f)

if __name__ == "__main__":
    main()
