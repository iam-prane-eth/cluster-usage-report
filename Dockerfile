FROM golang:1.24.0 AS builder

WORKDIR /app

COPY . .

RUN go build -o usage .

FROM python:3.12-slim

# Install Python & system dependencies
RUN apt-get update && \
    apt-get install -y python3 python3-pip fonts-dejavu-core fontconfig wget unzip ca-certificates \
                               fonts-dejavu-core \
                               fonts-montserrat \
                               fonts-lato \
                               fonts-symbola \
                               --no-install-recommends && \
    pip3 install markdown2 reportlab PyPDF2 beautifulsoup4 && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Install Source Sans Pro (direct TTF to expected folder)
RUN mkdir -p /usr/share/fonts/truetype/source-sans-pro && \
    wget -q https://github.com/adobe-fonts/source-sans/raw/release/TTF/SourceSans3-Regular.ttf \
        -O /usr/share/fonts/truetype/source-sans-pro/SourceSansPro-Regular.ttf && \
    wget -q https://github.com/adobe-fonts/source-sans/raw/release/TTF/SourceSans3-Semibold.ttf \
        -O /usr/share/fonts/truetype/source-sans-pro/SourceSansPro-Semibold.ttf && \
    fc-cache -f

COPY --from=builder /app/usage /usage

#COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY md_to_pdf.py /app/md_to_pdf.py

COPY bg_image.png /app/bg_image.png

COPY logo_image.png /app/logo_image.png

WORKDIR /app

CMD ["/usage"]

