// Chunked upload handling for kiss-drop

const CHUNK_SIZE = 5 * 1024 * 1024; // 5MB chunks - must match server

class ChunkedUploader {
    constructor(file, options = {}) {
        this.file = file;
        this.password = options.password || '';
        this.expiresIn = options.expiresIn || 'default';
        this.onProgress = options.onProgress || (() => {});
        this.onComplete = options.onComplete || (() => {});
        this.onError = options.onError || (() => {});

        this.uploadId = null;
        this.chunkSize = CHUNK_SIZE;
        this.totalChunks = Math.ceil(file.size / CHUNK_SIZE);
        this.uploadedChunks = 0;
        this.aborted = false;
    }

    async start() {
        try {
            // Initialize upload session
            const initResponse = await fetch('/api/upload/init', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    fileName: this.file.name,
                    fileSize: this.file.size,
                    password: this.password,
                    expiresIn: this.expiresIn
                })
            });

            if (!initResponse.ok) {
                throw new Error('Failed to initialize upload');
            }

            const initData = await initResponse.json();
            this.uploadId = initData.uploadId;
            this.chunkSize = initData.chunkSize;
            this.totalChunks = initData.totalChunks;

            // Upload chunks
            for (let i = 0; i < this.totalChunks; i++) {
                if (this.aborted) {
                    throw new Error('Upload aborted');
                }
                await this.uploadChunk(i);
            }

            // Complete the upload
            const completeResponse = await fetch(`/api/upload/${this.uploadId}/complete`, {
                method: 'POST'
            });

            if (!completeResponse.ok) {
                throw new Error('Failed to complete upload');
            }

            const result = await completeResponse.json();
            this.onComplete(result);

        } catch (error) {
            this.onError(error);
        }
    }

    async uploadChunk(index) {
        const start = index * this.chunkSize;
        const end = Math.min(start + this.chunkSize, this.file.size);
        const chunk = this.file.slice(start, end);

        const response = await fetch(`/api/upload/${this.uploadId}/chunk/${index}`, {
            method: 'POST',
            body: chunk
        });

        if (!response.ok) {
            throw new Error(`Failed to upload chunk ${index}`);
        }

        this.uploadedChunks++;
        const progress = (this.uploadedChunks / this.totalChunks) * 100;
        this.onProgress(progress, this.uploadedChunks, this.totalChunks);
    }

    abort() {
        this.aborted = true;
    }
}

// Use chunked upload for files larger than threshold
const CHUNKED_THRESHOLD = 10 * 1024 * 1024; // 10MB

function shouldUseChunkedUpload(file) {
    return file.size > CHUNKED_THRESHOLD;
}

// Export for use in templates
window.ChunkedUploader = ChunkedUploader;
window.shouldUseChunkedUpload = shouldUseChunkedUpload;
