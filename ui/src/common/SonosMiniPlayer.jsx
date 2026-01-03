import React, { useEffect, useState, useCallback } from 'react'
import { useTranslate } from 'react-admin'
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
    zIndex: 1300,
    backgroundColor: theme.palette.background.paper,
    borderTop: `1px solid ${theme.palette.divider}`,
    padding: theme.spacing(1, 2),
  },
  container: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
    maxWidth: 1200,
    margin: '0 auto',
  },
  deviceInfo: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    minWidth: 150,
  },
  trackInfo: {
    flex: 1,
    minWidth: 0,
    overflow: 'hidden',
  },
  trackTitle: {
    fontWeight: 500,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  trackArtist: {
    fontSize: '0.85em',
    color: theme.palette.text.secondary,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  controls: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
  },
  volumeControl: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    minWidth: 150,
  },
  volumeSlider: {
    width: 100,
  },
  stateIndicator: {
    fontSize: '0.75em',
    padding: theme.spacing(0.25, 1),
    borderRadius: theme.shape.borderRadius,
    backgroundColor: theme.palette.action.selected,
  },
  playing: {
    backgroundColor: theme.palette.success.main,
    color: theme.palette.success.contrastText,
  },
  paused: {
    backgroundColor: theme.palette.warning.main,
    color: theme.palette.warning.contrastText,
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
  },
  timeDisplay: {
    fontSize: '0.75em',
    color: theme.palette.text.secondary,
    minWidth: 45,
    textAlign: 'center',
  },
  qualityInfo: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
  },
  qualityBadge: {
    fontSize: '0.65em',
    padding: theme.spacing(0.25, 0.75),
    borderRadius: theme.shape.borderRadius,
    backgroundColor: theme.palette.action.selected,
    color: theme.palette.text.secondary,
    fontWeight: 500,
    whiteSpace: 'nowrap',
  },
  transcoding: {
    backgroundColor: theme.palette.warning.light,
    color: theme.palette.warning.contrastText,
  },
  hiRes: {
    backgroundColor: theme.palette.info.main,
    color: theme.palette.info.contrastText,
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

  const [visible, setVisible] = useState(false)
  const [activeDevice, setActiveDevice] = useState(null)
  const [playbackState, setPlaybackState] = useState(null)
  const [volume, setVolume] = useState(50)
  const [isSeeking, setIsSeeking] = useState(false)
  const [seekPosition, setSeekPosition] = useState(0)

  // Poll for active Sonos playback
  const checkPlayback = useCallback(async () => {
    try {
      const response = await httpClient('/api/cast/sonos/devices')
      const devices = response.json || []

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
  }, [])

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
            <SpeakerIcon color="primary" />
            <Box>
              <Typography variant="body2" className={classes.trackTitle}>
                {activeDevice?.roomName || 'Sonos'}
              </Typography>
              <Typography
                className={`${classes.stateIndicator} ${isPlaying ? classes.playing : isPaused ? classes.paused : ''}`}
              >
                {getStateLabel()}
              </Typography>
            </Box>
          </Box>

          {/* Track Info */}
          <Box className={classes.trackInfo}>
            {playbackState?.currentTrack ? (
              <>
                <Typography className={classes.trackTitle}>
                  {playbackState.currentTrack.title || 'Unknown Track'}
                </Typography>
                <Box style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <Typography className={classes.trackArtist}>
                    {playbackState.currentTrack.artist || 'Unknown Artist'}
                  </Typography>
                  {/* Quality Info */}
                  <Box className={classes.qualityInfo}>
                    {playbackState.currentTrack.format && (
                      <Typography
                        className={`${classes.qualityBadge} ${
                          playbackState.currentTrack.sampleRate > 44100 ? classes.hiRes : ''
                        }`}
                      >
                        {formatQuality(playbackState.currentTrack)}
                      </Typography>
                    )}
                    {playbackState.currentTrack.transcoding && (
                      <Typography className={`${classes.qualityBadge} ${classes.transcoding}`}>
                        Transcoding
                      </Typography>
                    )}
                  </Box>
                </Box>
              </>
            ) : (
              <Typography className={classes.trackArtist}>
                {isTransitioning ? 'Buffering...' : 'No track info'}
              </Typography>
            )}
          </Box>

          {/* Playback Controls */}
          <Box className={classes.controls}>
            <IconButton onClick={handlePrevious} size="small" title="Previous">
              <SkipPreviousIcon />
            </IconButton>
            {isPlaying ? (
              <IconButton onClick={handlePause} size="small" title="Pause">
                <PauseIcon />
              </IconButton>
            ) : (
              <IconButton onClick={handlePlay} size="small" title="Play">
                <PlayArrowIcon />
              </IconButton>
            )}
            <IconButton onClick={handleNext} size="small" title="Next">
              <SkipNextIcon />
            </IconButton>
            <IconButton onClick={handleStop} size="small" title="Stop">
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
            <VolumeOffIcon fontSize="small" style={{ opacity: 0.5 }} />
            <Slider
              className={classes.volumeSlider}
              value={volume}
              onChange={handleVolumeChange}
              min={0}
              max={100}
              size="small"
            />
            <VolumeUpIcon fontSize="small" style={{ opacity: 0.5 }} />
          </Box>

          {/* Close Button */}
          <IconButton onClick={handleClose} size="small">
            <CloseIcon fontSize="small" />
          </IconButton>
        </Box>
      </Paper>
    </Collapse>
  )
}

export default SonosMiniPlayer
