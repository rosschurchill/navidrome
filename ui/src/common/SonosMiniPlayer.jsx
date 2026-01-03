import React, { useEffect, useState, useCallback } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import { useTranslate } from 'react-admin'
import { setSonosActive, setSonosDevice } from '../actions'
import {
  Box,
  IconButton,
  Slider,
  Typography,
  Paper,
  Collapse,
} from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import PlayArrowIcon from '@material-ui/icons/PlayArrow'
import PauseIcon from '@material-ui/icons/Pause'
import StopIcon from '@material-ui/icons/Stop'
import SkipPreviousIcon from '@material-ui/icons/SkipPrevious'
import SkipNextIcon from '@material-ui/icons/SkipNext'
import VolumeUpIcon from '@material-ui/icons/VolumeUp'
import VolumeOffIcon from '@material-ui/icons/VolumeOff'
import SpeakerIcon from '@material-ui/icons/Speaker'
import CloseIcon from '@material-ui/icons/Close'
import httpClient from '../dataProvider/httpClient'

const useStyles = makeStyles((theme) => ({
  root: {
    position: 'fixed',
    bottom: 0,
    left: 0,
    right: 0,
    zIndex: 9999,
    background: 'linear-gradient(180deg, #282828 0%, #181818 100%)',
    borderTop: '1px solid #404040',
    padding: theme.spacing(0, 2),
    height: 56,
    display: 'flex',
    alignItems: 'center',
  },
  container: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    width: '100%',
    height: '100%',
  },
  deviceInfo: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.75),
    padding: theme.spacing(0.5, 1),
    backgroundColor: 'rgba(29, 185, 84, 0.1)',
    borderRadius: 4,
    border: '1px solid rgba(29, 185, 84, 0.3)',
  },
  deviceIcon: {
    color: '#1DB954',
    fontSize: 18,
  },
  deviceName: {
    color: '#1DB954',
    fontWeight: 600,
    fontSize: '0.8em',
  },
  deviceState: {
    color: '#1DB954',
    fontWeight: 500,
    fontSize: '0.8em',
    opacity: 0.85,
  },
  deviceSeparator: {
    color: '#1DB954',
    opacity: 0.5,
    fontSize: '0.8em',
  },
  trackInfo: {
    flex: '0 1 200px',
    minWidth: 120,
    overflow: 'hidden',
  },
  trackTitle: {
    fontWeight: 600,
    fontSize: '0.95em',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    color: '#ffffff',
  },
  trackArtist: {
    fontSize: '0.8em',
    color: '#b3b3b3',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  controls: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
  },
  controlButton: {
    color: '#b3b3b3',
    padding: 6,
    '&:hover': {
      color: '#ffffff',
      transform: 'scale(1.1)',
    },
    transition: 'all 0.1s ease',
  },
  playButton: {
    color: '#000000',
    backgroundColor: '#ffffff',
    padding: 6,
    '&:hover': {
      backgroundColor: '#ffffff',
      transform: 'scale(1.1)',
    },
    transition: 'all 0.1s ease',
  },
  stopButton: {
    color: '#b3b3b3',
    padding: 6,
    '&:hover': {
      color: '#ff5555',
      transform: 'scale(1.1)',
    },
    transition: 'all 0.1s ease',
  },
  volumeControl: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    minWidth: 140,
  },
  volumeIcon: {
    color: '#b3b3b3',
    fontSize: 20,
  },
  progressContainer: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    flex: 1,
    minWidth: 200,
  },
  progressSlider: {
    flex: 1,
    color: '#1DB954',
    height: 4,
    '& .MuiSlider-track': {
      backgroundColor: '#1DB954',
      height: 4,
    },
    '& .MuiSlider-rail': {
      backgroundColor: '#535353',
      height: 4,
    },
    '& .MuiSlider-thumb': {
      backgroundColor: '#ffffff',
      width: 10,
      height: 10,
      '&:hover': {
        boxShadow: '0 0 0 6px rgba(29, 185, 84, 0.16)',
      },
    },
  },
  volumeSlider: {
    width: 90,
    color: '#1DB954',
    '& .MuiSlider-track': {
      backgroundColor: '#1DB954',
    },
    '& .MuiSlider-rail': {
      backgroundColor: '#535353',
    },
    '& .MuiSlider-thumb': {
      backgroundColor: '#ffffff',
      width: 12,
      height: 12,
    },
  },
  timeDisplay: {
    fontSize: '0.7em',
    color: '#b3b3b3',
    minWidth: 32,
    textAlign: 'center',
    fontFamily: 'monospace',
  },
  qualityInfo: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
  },
  qualityBadge: {
    fontSize: '0.65em',
    padding: '2px 6px',
    borderRadius: 3,
    backgroundColor: '#404040',
    color: '#b3b3b3',
    fontWeight: 600,
    whiteSpace: 'nowrap',
  },
  transcoding: {
    backgroundColor: '#ff9800',
    color: '#000000',
  },
  hiRes: {
    backgroundColor: '#1DB954',
    color: '#000000',
  },
  closeButton: {
    color: '#b3b3b3',
    marginLeft: 'auto',
    '&:hover': {
      color: '#ffffff',
    },
  },
}))

// Format seconds as MM:SS
const formatTime = (seconds) => {
  if (!seconds || seconds < 0) return '0:00'
  const mins = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

// Format sample rate as kHz
const formatSampleRate = (hz) => {
  if (!hz) return null
  return hz >= 1000 ? `${(hz / 1000).toFixed(hz % 1000 === 0 ? 0 : 1)}kHz` : `${hz}Hz`
}

// Format quality string
const formatQuality = (track) => {
  if (!track) return null
  const parts = []
  if (track.format) parts.push(track.format)
  if (track.sampleRate) parts.push(formatSampleRate(track.sampleRate))
  if (track.bitDepth) parts.push(`${track.bitDepth}bit`)
  if (track.bitRate && track.format !== 'FLAC') parts.push(`${track.bitRate}kbps`)
  return parts.length > 0 ? parts.join(' / ') : null
}

export const SonosMiniPlayer = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const dispatch = useDispatch()

  // Get the device from Redux (set by SonosCastDialog when casting)
  const reduxDevice = useSelector((state) => state.player.sonosDevice)

  const [visible, setVisible] = useState(false)
  const [activeDevice, setActiveDevice] = useState(null)
  const [playbackState, setPlaybackState] = useState(null)
  const [volume, setVolume] = useState(50)
  const [isSeeking, setIsSeeking] = useState(false)
  const [seekPosition, setSeekPosition] = useState(0)

  // When Redux device changes (user cast to a new device), update local state
  useEffect(() => {
    if (reduxDevice) {
      setActiveDevice(reduxDevice)
    }
  }, [reduxDevice])

  // Notify Redux when Sonos player visibility changes
  useEffect(() => {
    dispatch(setSonosActive(visible))
  }, [visible, dispatch])

  // Poll for active Sonos playback
  const checkPlayback = useCallback(async () => {
    try {
      const response = await httpClient('/api/cast/sonos/devices')
      const devices = response.json || []

      // If we have an active device, check it first to maintain tracking
      if (activeDevice) {
        try {
          const stateResponse = await httpClient(`/api/cast/sonos/devices/${activeDevice.uuid}/state`)
          const state = stateResponse.json

          if (state.state === 'PLAYING' || state.state === 'PAUSED_PLAYBACK' || state.state === 'TRANSITIONING') {
            setPlaybackState(state)
            setVolume(state.volume || 50)
            setVisible(true)
            return
          }
        } catch (e) {
          // Current device no longer available, fall through to scan all
        }
      }

      // Check each device for active playback
      for (const device of devices) {
        try {
          const stateResponse = await httpClient(`/api/cast/sonos/devices/${device.uuid}/state`)
          const state = stateResponse.json

          if (state.state === 'PLAYING' || state.state === 'PAUSED_PLAYBACK' || state.state === 'TRANSITIONING') {
            setActiveDevice(device)
            setPlaybackState(state)
            setVolume(state.volume || 50)
            setVisible(true)
            return
          }
        } catch (e) {
          // Device state fetch failed, continue to next
        }
      }

      // No active playback found
      setVisible(false)
      setActiveDevice(null)
      setPlaybackState(null)
    } catch (error) {
      console.error('Failed to check Sonos playback:', error)
    }
  }, [activeDevice])

  useEffect(() => {
    // Initial check
    checkPlayback()

    // Poll faster when playing (1s) to update progress, slower when stopped (5s)
    const isPlaying = playbackState?.state === 'PLAYING'
    const pollInterval = isPlaying ? 1000 : 5000
    const interval = setInterval(checkPlayback, pollInterval)
    return () => clearInterval(interval)
  }, [checkPlayback, playbackState?.state])

  const handlePlay = async () => {
    if (!activeDevice) return
    try {
      await httpClient(`/api/cast/sonos/devices/${activeDevice.uuid}/play`, { method: 'POST' })
      checkPlayback()
    } catch (error) {
      console.error('Play failed:', error)
    }
  }

  const handlePause = async () => {
    if (!activeDevice) return
    try {
      await httpClient(`/api/cast/sonos/devices/${activeDevice.uuid}/pause`, { method: 'POST' })
      checkPlayback()
    } catch (error) {
      console.error('Pause failed:', error)
    }
  }

  const handleStop = async () => {
    if (!activeDevice) return
    try {
      await httpClient(`/api/cast/sonos/devices/${activeDevice.uuid}/stop`, { method: 'POST' })
      setVisible(false)
      setActiveDevice(null)
      setPlaybackState(null)
    } catch (error) {
      console.error('Stop failed:', error)
    }
  }

  const handleVolumeChange = async (event, newValue) => {
    setVolume(newValue)
    if (!activeDevice) return
    try {
      await httpClient(`/api/cast/sonos/devices/${activeDevice.uuid}/volume`, {
        method: 'POST',
        body: JSON.stringify({ volume: newValue }),
      })
    } catch (error) {
      console.error('Volume change failed:', error)
    }
  }

  const handleSeekStart = () => {
    setIsSeeking(true)
  }

  const handleSeekChange = (event, newValue) => {
    setSeekPosition(newValue)
  }

  const handleSeekEnd = async (event, newValue) => {
    setIsSeeking(false)
    if (!activeDevice) return
    try {
      await httpClient(`/api/cast/sonos/devices/${activeDevice.uuid}/seek`, {
        method: 'POST',
        body: JSON.stringify({ position: Math.floor(newValue) }),
      })
      checkPlayback()
    } catch (error) {
      console.error('Seek failed:', error)
    }
  }

  const handleNext = async () => {
    if (!activeDevice) return
    try {
      await httpClient(`/api/cast/sonos/devices/${activeDevice.uuid}/next`, { method: 'POST' })
      checkPlayback()
    } catch (error) {
      console.error('Next failed:', error)
    }
  }

  const handlePrevious = async () => {
    if (!activeDevice) return
    try {
      await httpClient(`/api/cast/sonos/devices/${activeDevice.uuid}/previous`, { method: 'POST' })
      checkPlayback()
    } catch (error) {
      console.error('Previous failed:', error)
    }
  }

  const handleClose = () => {
    setVisible(false)
  }

  const isPlaying = playbackState?.state === 'PLAYING'
  const isPaused = playbackState?.state === 'PAUSED_PLAYBACK'
  const isTransitioning = playbackState?.state === 'TRANSITIONING'

  const getStateLabel = () => {
    if (isPlaying) return 'Playing'
    if (isPaused) return 'Paused'
    if (isTransitioning) return 'Loading...'
    return playbackState?.state || 'Unknown'
  }

  return (
    <Collapse in={visible}>
      <Paper className={classes.root} elevation={8}>
        <Box className={classes.container}>
          {/* Device Info */}
          <Box className={classes.deviceInfo}>
            <SpeakerIcon className={classes.deviceIcon} />
            <Typography className={classes.deviceName}>
              {activeDevice?.roomName || 'Sonos'}
            </Typography>
            <Typography className={classes.deviceSeparator}>•</Typography>
            <Typography className={classes.deviceState}>
              {getStateLabel()}
            </Typography>
          </Box>

          {/* Track Info */}
          <Box className={classes.trackInfo}>
            {playbackState?.currentTrack ? (
              <>
                <Typography className={classes.trackTitle}>
                  {playbackState.currentTrack.title || 'Unknown Track'}
                </Typography>
                <Typography className={classes.trackArtist}>
                  {playbackState.currentTrack.artist || 'Unknown Artist'}
                </Typography>
              </>
            ) : (
              <Typography className={classes.trackArtist}>
                {isTransitioning ? 'Buffering...' : 'No track info'}
              </Typography>
            )}
          </Box>

          {/* Quality Info - separate section */}
          {playbackState?.currentTrack?.format && (
            <Box className={classes.qualityInfo}>
              <Typography
                className={`${classes.qualityBadge} ${
                  playbackState.currentTrack.sampleRate > 44100 ? classes.hiRes : ''
                }`}
              >
                {formatQuality(playbackState.currentTrack)}
              </Typography>
              {playbackState.currentTrack.transcoding && (
                <Typography className={`${classes.qualityBadge} ${classes.transcoding}`}>
                  Transcoding
                </Typography>
              )}
            </Box>
          )}

          {/* Playback Controls */}
          <Box className={classes.controls}>
            <IconButton onClick={handlePrevious} size="small" title="Previous" className={classes.controlButton}>
              <SkipPreviousIcon />
            </IconButton>
            {isPlaying ? (
              <IconButton onClick={handlePause} size="small" title="Pause" className={classes.playButton}>
                <PauseIcon />
              </IconButton>
            ) : (
              <IconButton onClick={handlePlay} size="small" title="Play" className={classes.playButton}>
                <PlayArrowIcon />
              </IconButton>
            )}
            <IconButton onClick={handleNext} size="small" title="Next" className={classes.controlButton}>
              <SkipNextIcon />
            </IconButton>
            <IconButton onClick={handleStop} size="small" title="Stop" className={classes.stopButton}>
              <StopIcon />
            </IconButton>
          </Box>

          {/* Progress Bar */}
          <Box className={classes.progressContainer}>
            <Typography className={classes.timeDisplay}>
              {formatTime(isSeeking ? seekPosition : playbackState?.currentTrack?.position)}
            </Typography>
            <Slider
              className={classes.progressSlider}
              value={isSeeking ? seekPosition : (playbackState?.currentTrack?.position || 0)}
              max={playbackState?.currentTrack?.duration || 100}
              onMouseDown={handleSeekStart}
              onChange={handleSeekChange}
              onChangeCommitted={handleSeekEnd}
              size="small"
            />
            <Typography className={classes.timeDisplay}>
              {formatTime(playbackState?.currentTrack?.duration)}
            </Typography>
          </Box>

          {/* Volume Control */}
          <Box className={classes.volumeControl}>
            <VolumeOffIcon className={classes.volumeIcon} />
            <Slider
              className={classes.volumeSlider}
              value={volume}
              onChange={handleVolumeChange}
              min={0}
              max={100}
              size="small"
            />
            <VolumeUpIcon className={classes.volumeIcon} />
          </Box>

          {/* Close Button */}
          <IconButton onClick={handleClose} size="small" className={classes.closeButton}>
            <CloseIcon />
          </IconButton>
        </Box>
      </Paper>
    </Collapse>
  )
}

export default SonosMiniPlayer
