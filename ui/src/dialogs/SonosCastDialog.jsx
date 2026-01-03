import React, { useEffect, useState } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import { useNotify, useTranslate } from 'react-admin'
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  ListItemSecondaryAction,
  IconButton,
  CircularProgress,
  Typography,
  Box,
  Slider,
  Tooltip,
} from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import SpeakerIcon from '@material-ui/icons/Speaker'
import VolumeUpIcon from '@material-ui/icons/VolumeUp'
import VolumeOffIcon from '@material-ui/icons/VolumeOff'
import RefreshIcon from '@material-ui/icons/Refresh'
import PlayArrowIcon from '@material-ui/icons/PlayArrow'
import { closeSonosCastDialog } from '../actions'
import httpClient from '../dataProvider/httpClient'

const useStyles = makeStyles((theme) => ({
  dialogContent: {
    minHeight: 200,
    paddingTop: theme.spacing(1),
  },
  deviceList: {
    width: '100%',
  },
  deviceItem: {
    borderRadius: theme.shape.borderRadius,
    marginBottom: theme.spacing(0.5),
    '&:hover': {
      backgroundColor: theme.palette.action.hover,
    },
  },
  selectedDevice: {
    backgroundColor: theme.palette.action.selected,
  },
  loadingContainer: {
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    minHeight: 150,
  },
  noDevices: {
    textAlign: 'center',
    padding: theme.spacing(4),
    color: theme.palette.text.secondary,
  },
  volumeSlider: {
    width: 100,
    marginRight: theme.spacing(1),
  },
  headerActions: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
  modelName: {
    color: theme.palette.text.secondary,
    fontSize: '0.85em',
  },
}))

export const SonosCastDialog = () => {
  const classes = useStyles()
  const dispatch = useDispatch()
  const notify = useNotify()
  const translate = useTranslate()

  const { open, selectedIds, resource, name } = useSelector(
    (state) => state.sonosCastDialog,
  )

  const [devices, setDevices] = useState([])
  const [loading, setLoading] = useState(false)
  const [casting, setCasting] = useState(false)
  const [selectedDevice, setSelectedDevice] = useState(null)
  const [refreshing, setRefreshing] = useState(false)

  const fetchDevices = async () => {
    setLoading(true)
    try {
      const response = await httpClient('/api/cast/sonos/devices')
      setDevices(response.json || [])
    } catch (error) {
      console.error('Failed to fetch Sonos devices:', error)
      notify('Failed to fetch Sonos devices', { type: 'warning' })
      setDevices([])
    } finally {
      setLoading(false)
    }
  }

  const refreshDevices = async () => {
    setRefreshing(true)
    try {
      await httpClient('/api/cast/sonos/devices/refresh', {
        method: 'POST',
      })
      await fetchDevices()
      notify('Sonos devices refreshed', { type: 'info' })
    } catch (error) {
      notify('Failed to refresh devices', { type: 'warning' })
    } finally {
      setRefreshing(false)
    }
  }

  const handleCast = async () => {
    if (!selectedDevice || !selectedIds?.length) {
      console.warn('Cast attempted without device or tracks', { selectedDevice, selectedIds })
      return
    }

    console.log('Starting cast', {
      device: selectedDevice.roomName,
      uuid: selectedDevice.uuid,
      trackCount: selectedIds.length,
      trackIds: selectedIds,
      resource: resource,
    })

    setCasting(true)
    try {
      const response = await httpClient(
        `/api/cast/sonos/devices/${selectedDevice.uuid}/cast`,
        {
          method: 'POST',
          body: JSON.stringify({
            trackIds: selectedIds,
            resource: resource,
          }),
        },
      )
      console.log('Cast response', response)
      notify(
        translate('message.sonosCastSuccess', {
          room: selectedDevice.roomName,
        }),
        { type: 'info' },
      )
      dispatch(closeSonosCastDialog())
    } catch (error) {
      console.error('Cast failed:', {
        error: error,
        message: error.message,
        status: error.status,
        body: error.body,
        device: selectedDevice.uuid,
        trackIds: selectedIds,
      })
      const errorMessage = error.body?.error || error.message || 'Unknown error'
      notify(
        translate('message.sonosCastFailure', { error: errorMessage }),
        { type: 'warning' },
      )
    } finally {
      setCasting(false)
    }
  }

  const handleVolumeChange = async (device, newVolume) => {
    try {
      await httpClient(
        `/api/cast/sonos/devices/${device.uuid}/volume`,
        {
          method: 'POST',
          body: JSON.stringify({ volume: newVolume }),
        },
      )
      setDevices((prev) =>
        prev.map((d) =>
          d.uuid === device.uuid ? { ...d, volume: newVolume } : d,
        ),
      )
    } catch (error) {
      notify('Failed to set volume', { type: 'warning' })
    }
  }

  const handleClose = (e) => {
    dispatch(closeSonosCastDialog())
    e?.stopPropagation()
  }

  useEffect(() => {
    if (open) {
      fetchDevices()
      setSelectedDevice(null)
    }
  }, [open])

  const getDeviceIcon = (device) => {
    return <SpeakerIcon />
  }

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      aria-labelledby="sonos-cast-dialog"
      fullWidth
      maxWidth="sm"
    >
      <DialogTitle id="sonos-cast-dialog">
        <Box display="flex" justifyContent="space-between" alignItems="center">
          <span>
            {translate('message.castToSonos', {
              name: name || translate(`resources.${resource}.name`, { smart_count: selectedIds?.length }),
              smart_count: selectedIds?.length,
            })}
          </span>
          <Tooltip title={translate('message.refreshDevices')}>
            <IconButton
              onClick={refreshDevices}
              disabled={refreshing}
              size="small"
            >
              {refreshing ? (
                <CircularProgress size={20} />
              ) : (
                <RefreshIcon />
              )}
            </IconButton>
          </Tooltip>
        </Box>
      </DialogTitle>

      <DialogContent className={classes.dialogContent}>
        {loading ? (
          <Box className={classes.loadingContainer}>
            <CircularProgress />
          </Box>
        ) : devices.length === 0 ? (
          <Box className={classes.noDevices}>
            <SpeakerIcon style={{ fontSize: 48, opacity: 0.5 }} />
            <Typography variant="body1" style={{ marginTop: 16 }}>
              {translate('message.noSonosDevices')}
            </Typography>
            <Typography variant="body2">
              {translate('message.noSonosDevicesHint')}
            </Typography>
          </Box>
        ) : (
          <List className={classes.deviceList}>
            {devices.map((device) => (
              <ListItem
                key={device.uuid}
                button
                onClick={() => setSelectedDevice(device)}
                className={`${classes.deviceItem} ${
                  selectedDevice?.uuid === device.uuid
                    ? classes.selectedDevice
                    : ''
                }`}
              >
                <ListItemIcon>{getDeviceIcon(device)}</ListItemIcon>
                <ListItemText
                  primary={device.roomName}
                  secondary={
                    <span className={classes.modelName}>
                      {device.modelName}
                    </span>
                  }
                />
                <ListItemSecondaryAction>
                  <Box display="flex" alignItems="center">
                    <VolumeOffIcon
                      fontSize="small"
                      style={{ opacity: 0.5, marginRight: 8 }}
                    />
                    <Slider
                      className={classes.volumeSlider}
                      value={device.volume || 0}
                      onChange={(e, val) => handleVolumeChange(device, val)}
                      min={0}
                      max={100}
                      size="small"
                    />
                    <VolumeUpIcon
                      fontSize="small"
                      style={{ opacity: 0.5, marginLeft: 8 }}
                    />
                  </Box>
                </ListItemSecondaryAction>
              </ListItem>
            ))}
          </List>
        )}
      </DialogContent>

      <DialogActions>
        <Button onClick={handleClose} color="primary">
          {translate('ra.action.cancel')}
        </Button>
        <Button
          onClick={handleCast}
          color="primary"
          variant="contained"
          disabled={!selectedDevice || casting || loading}
          startIcon={
            casting ? <CircularProgress size={16} /> : <PlayArrowIcon />
          }
        >
          {casting
            ? translate('message.casting')
            : translate('message.castNow')}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
