# Use an official Go image as the base
FROM gocv/opencv:latest

# Set the working directory inside the container
WORKDIR /app

RUN apt-get update -qq

# Copy the Go modules manifest and download the modules
COPY go.mod go.sum ./
RUN go mod download

RUN apt-get install -y -qq libtesseract-dev libleptonica-dev

# In case you face TESSDATA_PREFIX error, you minght need to set env vars
# to specify the directory where "tessdata" is located.
ENV TESSDATA_PREFIX=/usr/share/tesseract-ocr/5/tessdata/

# Load languages.
# These {lang}.traineddata would b located under ${TESSDATA_PREFIX}/tessdata.
RUN apt-get install -y -qq \
  tesseract-ocr-eng \
  tesseract-ocr-deu \
  tesseract-ocr-jpn

# Copy the rest of the application code
COPY . .

# Compile the Go application
RUN go build -o main .

# Run the Go application
CMD ["./main"]
