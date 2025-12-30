import React, { useState, useEffect, useCallback } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import { useNotify, useRefresh, useTranslate } from 'react-admin'
import {
  Button,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  ListItemSecondaryAction,
  Typography,
  CircularProgress,
  Chip,
  TextField,
  makeStyles,
  IconButton,
  Collapse,
} from '@material-ui/core'
import ExpandMoreIcon from '@material-ui/icons/ExpandMore'
import ExpandLessIcon from '@material-ui/icons/ExpandLess'
import WarningIcon from '@material-ui/icons/Warning'
import { closeSplitAlbumsDialog } from '../actions'
import { httpClient } from '../dataProvider'
import { REST_URL } from '../consts'

const useStyles = makeStyles((theme) => ({
  dialogPaper: {
    minHeight: '60vh',
    maxHeight: '80vh',
  },
  dialogContent: {
    paddingTop: theme.spacing(1),
  },
  listItem: {
    borderBottom: `1px solid ${theme.palette.divider}`,
    flexDirection: 'column',
    alignItems: 'flex-start',
  },
  listItemMain: {
    display: 'flex',
    width: '100%',
    alignItems: 'center',
  },
  albumInfo: {
    flex: 1,
  },
  albumName: {
    fontWeight: 'bold',
  },
  chip: {
    marginLeft: theme.spacing(1),
  },
  compilationChip: {
    backgroundColor: theme.palette.warning.light,
  },
  artistList: {
    paddingLeft: theme.spacing(4),
    width: '100%',
  },
  artistItem: {
    fontSize: '0.875rem',
    color: theme.palette.text.secondary,
  },
  fixInput: {
    marginTop: theme.spacing(1),
    marginLeft: theme.spacing(4),
    width: 'calc(100% - 32px)',
  },
  loading: {
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    minHeight: 200,
  },
  noIssues: {
    textAlign: 'center',
    padding: theme.spacing(4),
    color: theme.palette.text.secondary,
  },
  warningIcon: {
    color: theme.palette.warning.main,
    marginRight: theme.spacing(1),
  },
}))

export const SplitAlbumsDialog = () => {
  const classes = useStyles()
  const { open } = useSelector((state) => state.splitAlbumsDialog)
  const dispatch = useDispatch()
  const translate = useTranslate()
  const notify = useNotify()
  const refresh = useRefresh()

  const [loading, setLoading] = useState(false)
  const [splitAlbums, setSplitAlbums] = useState([])
  const [selected, setSelected] = useState({})
  const [expanded, setExpanded] = useState({})
  const [fixes, setFixes] = useState({})
  const [merging, setMerging] = useState(false)

  const fetchSplitAlbums = useCallback(async () => {
    setLoading(true)
    try {
      const response = await httpClient(`${REST_URL}/splitAlbums`)
      const data = response.json || []
      setSplitAlbums(data)
      // Initialize fixes with suggested values
      const initialFixes = {}
      data.forEach((album, idx) => {
        initialFixes[idx] = album.suggestedFix || ''
      })
      setFixes(initialFixes)
    } catch (error) {
      console.error('Error fetching split albums:', error)
      notify('Error loading split albums', { type: 'error' })
    } finally {
      setLoading(false)
    }
  }, [notify])

  useEffect(() => {
    if (open) {
      fetchSplitAlbums()
      setSelected({})
      setExpanded({})
    }
  }, [open, fetchSplitAlbums])

  const handleClose = () => {
    dispatch(closeSplitAlbumsDialog())
  }

  const handleToggleSelect = (idx) => {
    setSelected((prev) => ({
      ...prev,
      [idx]: !prev[idx],
    }))
  }

  const handleToggleExpand = (idx) => {
    setExpanded((prev) => ({
      ...prev,
      [idx]: !prev[idx],
    }))
  }

  const handleFixChange = (idx, value) => {
    setFixes((prev) => ({
      ...prev,
      [idx]: value,
    }))
  }

  const handleMerge = async () => {
    const selectedAlbums = Object.entries(selected)
      .filter(([_, isSelected]) => isSelected)
      .map(([idx]) => parseInt(idx))

    if (selectedAlbums.length === 0) {
      notify('Please select at least one album to fix', { type: 'warning' })
      return
    }

    setMerging(true)
    let successCount = 0
    let errorCount = 0

    for (const idx of selectedAlbums) {
      const album = splitAlbums[idx]
      const targetArtist = fixes[idx]

      if (!targetArtist) {
        notify(`No target artist specified for "${album.name}"`, { type: 'warning' })
        errorCount++
        continue
      }

      try {
        await httpClient(`${REST_URL}/splitAlbums/merge`, {
          method: 'POST',
          body: JSON.stringify({
            albumIds: album.albumIds,
            targetAlbumArtist: targetArtist,
          }),
        })
        successCount++
      } catch (error) {
        console.error(`Error merging album "${album.name}":`, error)
        errorCount++
      }
    }

    setMerging(false)

    if (successCount > 0) {
      notify(`Successfully merged ${successCount} album(s). A rescan may be needed.`, { type: 'success' })
      refresh()
      fetchSplitAlbums()
      setSelected({})
    }

    if (errorCount > 0) {
      notify(`Failed to merge ${errorCount} album(s)`, { type: 'error' })
    }
  }

  const selectedCount = Object.values(selected).filter(Boolean).length

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      aria-labelledby="split-albums-dialog"
      fullWidth
      maxWidth="md"
      classes={{ paper: classes.dialogPaper }}
    >
      <DialogTitle id="split-albums-dialog">
        <WarningIcon className={classes.warningIcon} />
        {translate('resources.album.splitAlbums.title', {
          _: 'Split Albums Detected',
        })}
      </DialogTitle>
      <DialogContent className={classes.dialogContent}>
        {loading ? (
          <div className={classes.loading}>
            <CircularProgress />
          </div>
        ) : splitAlbums.length === 0 ? (
          <div className={classes.noIssues}>
            <Typography variant="h6">
              {translate('resources.album.splitAlbums.noIssues', {
                _: 'No split albums detected',
              })}
            </Typography>
            <Typography variant="body2">
              {translate('resources.album.splitAlbums.noIssuesDesc', {
                _: 'All albums appear to be correctly grouped.',
              })}
            </Typography>
          </div>
        ) : (
          <>
            <Typography variant="body2" gutterBottom>
              {translate('resources.album.splitAlbums.description', {
                _: 'The following albums have been split into multiple entries due to different album artists. Select the albums you want to merge and specify the correct album artist.',
              })}
            </Typography>
            <List>
              {splitAlbums.map((album, idx) => (
                <ListItem
                  key={idx}
                  className={classes.listItem}
                  dense
                >
                  <div className={classes.listItemMain}>
                    <ListItemIcon>
                      <Checkbox
                        checked={!!selected[idx]}
                        onChange={() => handleToggleSelect(idx)}
                        color="primary"
                      />
                    </ListItemIcon>
                    <ListItemText
                      className={classes.albumInfo}
                      primary={
                        <span className={classes.albumName}>
                          {album.name}
                          <Chip
                            size="small"
                            label={`${album.splitCount} splits`}
                            className={classes.chip}
                          />
                          <Chip
                            size="small"
                            label={`${album.totalTracks} tracks`}
                            className={classes.chip}
                          />
                          {album.isCompilation && (
                            <Chip
                              size="small"
                              label="Compilation"
                              className={`${classes.chip} ${classes.compilationChip}`}
                            />
                          )}
                        </span>
                      }
                    />
                    <IconButton
                      size="small"
                      onClick={() => handleToggleExpand(idx)}
                    >
                      {expanded[idx] ? <ExpandLessIcon /> : <ExpandMoreIcon />}
                    </IconButton>
                  </div>
                  <Collapse in={expanded[idx]} timeout="auto" unmountOnExit>
                    <div className={classes.artistList}>
                      <Typography variant="caption" color="textSecondary">
                        Current album artists:
                      </Typography>
                      {album.albumArtists.map((artist, artistIdx) => (
                        <Typography
                          key={artistIdx}
                          className={classes.artistItem}
                        >
                          • {artist}
                        </Typography>
                      ))}
                    </div>
                  </Collapse>
                  {selected[idx] && (
                    <TextField
                      className={classes.fixInput}
                      label={translate('resources.album.splitAlbums.targetArtist', {
                        _: 'Merge under album artist',
                      })}
                      value={fixes[idx] || ''}
                      onChange={(e) => handleFixChange(idx, e.target.value)}
                      variant="outlined"
                      size="small"
                      fullWidth
                      helperText={`Suggested: "${album.suggestedFix}"${album.isCompilation ? ' (compilation)' : ''}`}
                    />
                  )}
                </ListItem>
              ))}
            </List>
          </>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} color="default">
          {translate('ra.action.cancel')}
        </Button>
        <Button
          onClick={handleMerge}
          color="primary"
          variant="contained"
          disabled={selectedCount === 0 || merging}
          startIcon={merging && <CircularProgress size={16} />}
        >
          {merging
            ? translate('resources.album.splitAlbums.merging', { _: 'Merging...' })
            : translate('resources.album.splitAlbums.merge', {
                _: `Merge ${selectedCount} Album(s)`,
                smart_count: selectedCount,
              })}
        </Button>
      </DialogActions>
    </Dialog>
  )
}

export default SplitAlbumsDialog
