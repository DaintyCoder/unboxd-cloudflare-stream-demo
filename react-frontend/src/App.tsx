import { useState, useEffect } from 'react'
import './App.css'

interface VideoStatus {
  state: string;
  errorReasonCode: string;
  errorReasonText: string;
}

interface UploadResult {
  uid: string;
  preview: string;
  status: VideoStatus;
  readyToStream: boolean;
  thumbnail: string;
  playback: {
    hls: string;
    dash: string;
  };
}

interface UploadResponse {
  result: UploadResult;
  success: boolean;
  errors: any[];
}

function App() {
  const [file, setFile] = useState<File | null>(null)
  const [uploading, setUploading] = useState(false)
  const [videoData, setVideoData] = useState<UploadResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [polling, setPolling] = useState(false)

  useEffect(() => {
    let pollInterval: NodeJS.Timeout | null = null;

    const checkVideoStatus = async (uid: string) => {
      try {
        const response = await fetch(`http://localhost:3000/api/video/${uid}`);
        const data: UploadResponse = await response.json();

        if (data.success && data.result) {
          setVideoData(data.result);
          
          // If video is ready or failed, stop polling
          if (data.result.readyToStream || 
              data.result.status.state === 'failed' || 
              data.result.status.errorReasonCode) {
            setPolling(false);
          }
        } else {
          throw new Error('Failed to get video status');
        }
      } catch (err) {
        console.error('Error checking video status:', err);
        setPolling(false);
      }
    };

    // Start polling if we have a video UID and it's not ready
    if (videoData?.uid && !videoData.readyToStream && polling) {
      pollInterval = setInterval(() => {
        checkVideoStatus(videoData.uid);
      }, 5000); // Check every 5 seconds
    }

    return () => {
      if (pollInterval) {
        clearInterval(pollInterval);
      }
    };
  }, [videoData?.uid, polling]);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      setFile(e.target.files[0])
      setError(null)
    }
  }

  const handleUpload = async () => {
    if (!file) {
      setError('Please select a file first')
      return
    }

    setUploading(true)
    setError(null)
    setVideoData(null)

    const formData = new FormData()
    formData.append('video', file)

    try {
      const response = await fetch('http://localhost:3000/api/upload', {
        method: 'POST',
        body: formData,
      })

      const data: UploadResponse = await response.json()
      
      if (data.success && data.result) {
        setVideoData(data.result)
        setPolling(true) // Start polling for status
      } else {
        throw new Error('Upload failed: ' + JSON.stringify(data.errors))
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setUploading(false)
    }
  }

  const getStatusDisplay = () => {
    if (!videoData) return null;
    
    switch (videoData.status.state) {
      case 'queued':
        return 'Video is queued for processing...';
      case 'downloading':
        return 'Downloading video...';
      case 'processing':
        return 'Processing video...';
      case 'ready':
        return 'Video is ready to play';
      case 'failed':
        return `Processing failed: ${videoData.status.errorReasonText || 'Unknown error'}`;
      default:
        return `Status: ${videoData.status.state}`;
    }
  }

  return (
    <div className="max-w-2xl mx-auto p-4">
      <h1 className="text-2xl font-bold mb-4">Video Upload Demo</h1>
      
      <div className="mb-4">
        <input
          type="file"
          accept="video/*"
          onChange={handleFileChange}
          className="mb-2 block w-full text-sm text-gray-500
            file:mr-4 file:py-2 file:px-4
            file:rounded-full file:border-0
            file:text-sm file:font-semibold
            file:bg-blue-50 file:text-blue-700
            hover:file:bg-blue-100"
        />
        
        <button
          onClick={handleUpload}
          disabled={uploading || !file}
          className={`w-full py-2 px-4 rounded ${
            uploading
              ? 'bg-gray-400'
              : 'bg-blue-500 hover:bg-blue-600'
          } text-white font-semibold`}
        >
          {uploading ? 'Uploading...' : 'Upload Video'}
        </button>
      </div>

      {error && (
        <div className="p-4 mb-4 text-red-700 bg-red-100 rounded">
          {error}
        </div>
      )}

      {videoData && (
        <div className="mt-4">
          <h2 className="text-xl font-semibold mb-2">Video Status</h2>
          <p className="mb-2">{getStatusDisplay()}</p>
          
          {!videoData.readyToStream && videoData.thumbnail && (
            <div className="p-4 bg-yellow-50 text-yellow-700 rounded">
              <p>Video is being processed. You'll be able to play it once it's ready.</p>
              <img 
                src={videoData.thumbnail} 
                alt="Video thumbnail" 
                className="mt-2 rounded w-full"
              />
            </div>
          )}

          {videoData.readyToStream && (
            <div>
              <iframe
                src={videoData.preview}
                className="w-full aspect-video rounded"
                title="Uploaded video preview"
                frameBorder="0"
                allow="accelerometer; autoplay; encrypted-media; gyroscope; picture-in-picture"
                allowFullScreen
              />
              <div className="mt-2 p-4 bg-gray-50 rounded">
                <p className="text-sm text-gray-600">
                  HLS URL: {videoData.playback.hls}<br/>
                  DASH URL: {videoData.playback.dash}
                </p>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

export default App