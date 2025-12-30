import React, { useState } from 'react'
import Table from '@material-ui/core/Table'
import TableBody from '@material-ui/core/TableBody'
import TableCell from '@material-ui/core/TableCell'
import TableContainer from '@material-ui/core/TableContainer'
import TableRow from '@material-ui/core/TableRow'
import {
  BooleanField,
  DateField,
  TextField,
  NumberField,
  FunctionField,
  useTranslate,
  useRecordContext,
} from 'react-admin'
import { humanize, underscore } from 'inflection'
import {
  ArtistLinkField,
  BitrateField,
  ParticipantsInfo,
  SizeField,
} from './index'
import { MultiLineTextField } from './MultiLineTextField'
import { makeStyles } from '@material-ui/core/styles'
import config from '../config'
import { AlbumLinkField } from '../song/AlbumLinkField'
import { Tab, Tabs, Typography, Divider, Box, Chip, IconButton, Tooltip } from '@material-ui/core'
import FileCopyIcon from '@material-ui/icons/FileCopy'

const useStyles = makeStyles((theme) => ({
  gain: {
    '&:after': {
      content: (props) => (props.gain ? " ' db'" : ''),
    },
  },
  tableCell: {
    width: '17.5%',
    fontWeight: 500,
    color: theme.palette.text.secondary,
    verticalAlign: 'top',
    paddingTop: theme.spacing(1),
    paddingBottom: theme.spacing(1),
  },
  value: {
    whiteSpace: 'pre-line',
    wordBreak: 'break-word',
  },
  sectionHeader: {
    backgroundColor: theme.palette.action.hover,
    padding: theme.spacing(1, 2),
    marginTop: theme.spacing(2),
    marginBottom: theme.spacing(1),
    borderRadius: theme.shape.borderRadius,
  },
  pathCell: {
    fontFamily: 'monospace',
    fontSize: '0.85rem',
    backgroundColor: theme.palette.action.hover,
    padding: theme.spacing(1),
    borderRadius: theme.shape.borderRadius,
    wordBreak: 'break-all',
  },
  chip: {
    margin: theme.spacing(0.5),
  },
  idField: {
    fontFamily: 'monospace',
    fontSize: '0.8rem',
    color: theme.palette.text.secondary,
  },
  copyButton: {
    marginLeft: theme.spacing(1),
    padding: 4,
  },
  qualityBadge: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
}))

const SectionHeader = ({ children, classes }) => (
  <TableRow>
    <TableCell colSpan={2} className={classes.sectionHeader}>
      <Typography variant="subtitle2">{children}</Typography>
    </TableCell>
  </TableRow>
)

const InfoRow = ({ label, value, classes, translate, copyable = false }) => {
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    const textValue = typeof value === 'string' ? value : value?.props?.children || ''
    navigator.clipboard.writeText(textValue)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  if (value === null || value === undefined || value === '') return null

  return (
    <TableRow>
      <TableCell scope="row" className={classes.tableCell}>
        {translate ? translate(`resources.song.fields.${label}`, { _: humanize(underscore(label)) }) : label}:
      </TableCell>
      <TableCell align="left" className={classes.value}>
        {value}
        {copyable && (
          <Tooltip title={copied ? "Copied!" : "Copy"}>
            <IconButton size="small" onClick={handleCopy} className={classes.copyButton}>
              <FileCopyIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        )}
      </TableCell>
    </TableRow>
  )
}

const formatDuration = (seconds) => {
  if (!seconds) return ''
  const mins = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

const formatBytes = (bytes) => {
  if (!bytes) return ''
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  while (bytes >= 1024 && i < units.length - 1) {
    bytes /= 1024
    i++
  }
  return `${bytes.toFixed(2)} ${units[i]}`
}

const getQualityLabel = (record) => {
  const { bitRate, sampleRate, bitDepth, suffix } = record
  const isLossless = ['flac', 'alac', 'wav', 'aiff', 'ape', 'dsd', 'dsf'].includes(suffix?.toLowerCase())
  const isHiRes = sampleRate > 48000 || bitDepth > 16

  if (isLossless && isHiRes) return { label: 'Hi-Res Lossless', color: 'secondary' }
  if (isLossless) return { label: 'Lossless', color: 'primary' }
  if (bitRate >= 320) return { label: 'High Quality', color: 'default' }
  return { label: 'Compressed', color: 'default' }
}

export const SongInfo = (props) => {
  const classes = useStyles({ gain: config.enableReplayGain })
  const translate = useTranslate()
  const record = useRecordContext(props)
  const [tab, setTab] = useState(0)

  if (!record) return null

  const quality = getQualityLabel(record)

  // Extract filename from path
  const filename = record.path?.split('/').pop() || ''
  const directory = record.path?.substring(0, record.path.lastIndexOf('/')) || ''

  // These are already displayed in other fields or are album-level tags
  const excludedTags = [
    'genre',
    'disctotal',
    'tracktotal',
    'releasetype',
    'recordlabel',
    'media',
    'albumversion',
  ]

  const tags = Object.entries(record.tags ?? {}).filter(
    (tag) => !excludedTags.includes(tag[0]),
  )

  const roles = []
  for (const name of Object.keys(record.participants || {})) {
    if (name === 'albumartist' || name === 'artist') continue
    roles.push([name, record.participants[name].length])
  }

  return (
    <TableContainer>
      <Tabs value={tab} onChange={(_, value) => setTab(value)}>
        <Tab label="Overview" id="overview-tab" />
        <Tab label="Technical" id="technical-tab" />
        <Tab label="IDs & Tags" id="ids-tab" />
        {record.rawTags && <Tab label="Raw Tags" id="raw-tags-tab" />}
      </Tabs>

      {/* OVERVIEW TAB */}
      <div hidden={tab !== 0} id="overview-body">
        <Table size="small">
          <TableBody>
            <SectionHeader classes={classes}>📁 File Location</SectionHeader>
            <InfoRow
              label="File Name"
              value={<span className={classes.pathCell}>{filename}</span>}
              classes={classes}
              copyable
            />
            <InfoRow
              label="Directory"
              value={<span className={classes.pathCell}>{directory}</span>}
              classes={classes}
              copyable
            />
            <InfoRow
              label="Full Path"
              value={<span className={classes.pathCell}>{record.path}</span>}
              classes={classes}
              copyable
            />
            <InfoRow
              label="Library"
              value={record.libraryName}
              classes={classes}
            />

            <SectionHeader classes={classes}>🎵 Track Information</SectionHeader>
            <InfoRow
              label="Title"
              value={record.title}
              classes={classes}
              translate={translate}
            />
            <InfoRow
              label="Artist"
              value={<ArtistLinkField source="artist" record={record} limit={Infinity} />}
              classes={classes}
              translate={translate}
            />
            <InfoRow
              label="Album"
              value={<AlbumLinkField source="album" sortByOrder={'ASC'} record={record} />}
              classes={classes}
              translate={translate}
            />
            <InfoRow
              label="Album Artist"
              value={<ArtistLinkField source="albumArtist" record={record} limit={Infinity} />}
              classes={classes}
              translate={translate}
            />
            <InfoRow
              label="Track"
              value={record.trackNumber ? `${record.trackNumber}${record.discNumber > 1 ? ` (Disc ${record.discNumber})` : ''}` : null}
              classes={classes}
            />
            {record.discSubtitle && (
              <InfoRow
                label="Disc Subtitle"
                value={record.discSubtitle}
                classes={classes}
              />
            )}
            <InfoRow
              label="Genre"
              value={record.genres?.map((g) => g.name).join(' • ')}
              classes={classes}
              translate={translate}
            />
            <InfoRow
              label="Year"
              value={record.year || record.releaseYear}
              classes={classes}
            />
            {record.compilation && (
              <InfoRow
                label="Compilation"
                value={<BooleanField source="compilation" record={record} />}
                classes={classes}
                translate={translate}
              />
            )}
            {record.comment && (
              <InfoRow
                label="Comment"
                value={<MultiLineTextField source="comment" record={record} />}
                classes={classes}
                translate={translate}
              />
            )}

            <SectionHeader classes={classes}>📊 Play Statistics</SectionHeader>
            <InfoRow
              label="Play Count"
              value={record.playCount || 0}
              classes={classes}
              translate={translate}
            />
            {record.playCount > 0 && (
              <InfoRow
                label="Last Played"
                value={<DateField record={record} source="playDate" showTime />}
                classes={classes}
              />
            )}
            <InfoRow
              label="Added to Library"
              value={<DateField source="createdAt" record={record} showTime />}
              classes={classes}
            />
            <InfoRow
              label="File Modified"
              value={<DateField source="updatedAt" record={record} showTime />}
              classes={classes}
            />

            <ParticipantsInfo classes={classes} record={record} />
          </TableBody>
        </Table>
      </div>

      {/* TECHNICAL TAB */}
      <div hidden={tab !== 1} id="technical-body">
        <Table size="small">
          <TableBody>
            <SectionHeader classes={classes}>🎚️ Audio Quality</SectionHeader>
            <TableRow>
              <TableCell className={classes.tableCell}>Quality:</TableCell>
              <TableCell>
                <Box className={classes.qualityBadge}>
                  <Chip label={quality.label} color={quality.color} size="small" />
                  <Typography variant="body2" color="textSecondary">
                    {record.suffix?.toUpperCase()}
                  </Typography>
                </Box>
              </TableCell>
            </TableRow>
            <InfoRow
              label="Format"
              value={record.suffix?.toUpperCase()}
              classes={classes}
            />
            <InfoRow
              label="Duration"
              value={formatDuration(record.duration)}
              classes={classes}
            />
            <InfoRow
              label="Bit Rate"
              value={record.bitRate ? `${record.bitRate} kbps` : null}
              classes={classes}
            />
            <InfoRow
              label="Sample Rate"
              value={record.sampleRate ? `${record.sampleRate} Hz` : null}
              classes={classes}
            />
            <InfoRow
              label="Bit Depth"
              value={record.bitDepth ? `${record.bitDepth}-bit` : null}
              classes={classes}
            />
            <InfoRow
              label="Channels"
              value={record.channels === 1 ? 'Mono' : record.channels === 2 ? 'Stereo' : record.channels ? `${record.channels} channels` : null}
              classes={classes}
            />

            <SectionHeader classes={classes}>📦 File Details</SectionHeader>
            <InfoRow
              label="File Size"
              value={formatBytes(record.size)}
              classes={classes}
            />
            <InfoRow
              label="Has Cover Art"
              value={record.hasCoverArt ? 'Yes' : 'No'}
              classes={classes}
            />

            {config.enableReplayGain && (
              <>
                <SectionHeader classes={classes}>🔊 ReplayGain</SectionHeader>
                <InfoRow
                  label="Track Gain"
                  value={record.rgTrackGain !== null ? `${record.rgTrackGain?.toFixed(2)} dB` : 'Not set'}
                  classes={classes}
                />
                <InfoRow
                  label="Track Peak"
                  value={record.rgTrackPeak !== null ? record.rgTrackPeak?.toFixed(6) : 'Not set'}
                  classes={classes}
                />
                <InfoRow
                  label="Album Gain"
                  value={record.rgAlbumGain !== null ? `${record.rgAlbumGain?.toFixed(2)} dB` : 'Not set'}
                  classes={classes}
                />
                <InfoRow
                  label="Album Peak"
                  value={record.rgAlbumPeak !== null ? record.rgAlbumPeak?.toFixed(6) : 'Not set'}
                  classes={classes}
                />
              </>
            )}

            {record.bpm && (
              <>
                <SectionHeader classes={classes}>🎹 Music Analysis</SectionHeader>
                <InfoRow
                  label="BPM"
                  value={record.bpm}
                  classes={classes}
                />
              </>
            )}
          </TableBody>
        </Table>
      </div>

      {/* IDS & TAGS TAB */}
      <div hidden={tab !== 2} id="ids-body">
        <Table size="small">
          <TableBody>
            <SectionHeader classes={classes}>🔑 Internal IDs</SectionHeader>
            <InfoRow
              label="Track ID"
              value={<span className={classes.idField}>{record.id}</span>}
              classes={classes}
              copyable
            />
            <InfoRow
              label="Album ID"
              value={<span className={classes.idField}>{record.albumId}</span>}
              classes={classes}
              copyable
            />
            <InfoRow
              label="Artist ID"
              value={<span className={classes.idField}>{record.artistId}</span>}
              classes={classes}
              copyable
            />
            <InfoRow
              label="Folder ID"
              value={<span className={classes.idField}>{record.folderId}</span>}
              classes={classes}
              copyable
            />

            {(record.mbzRecordingID || record.mbzReleaseTrackID || record.mbzAlbumID || record.mbzArtistID) && (
              <>
                <SectionHeader classes={classes}>🎼 MusicBrainz IDs</SectionHeader>
                <InfoRow
                  label="Recording ID"
                  value={record.mbzRecordingID ? <span className={classes.idField}>{record.mbzRecordingID}</span> : null}
                  classes={classes}
                  copyable
                />
                <InfoRow
                  label="Release Track ID"
                  value={record.mbzReleaseTrackID ? <span className={classes.idField}>{record.mbzReleaseTrackID}</span> : null}
                  classes={classes}
                  copyable
                />
                <InfoRow
                  label="Album ID"
                  value={record.mbzAlbumID ? <span className={classes.idField}>{record.mbzAlbumID}</span> : null}
                  classes={classes}
                  copyable
                />
                <InfoRow
                  label="Release Group ID"
                  value={record.mbzReleaseGroupID ? <span className={classes.idField}>{record.mbzReleaseGroupID}</span> : null}
                  classes={classes}
                  copyable
                />
                <InfoRow
                  label="Artist ID"
                  value={record.mbzArtistID ? <span className={classes.idField}>{record.mbzArtistID}</span> : null}
                  classes={classes}
                  copyable
                />
                <InfoRow
                  label="Album Type"
                  value={record.mbzAlbumType}
                  classes={classes}
                />
              </>
            )}

            {record.catalogNum && (
              <>
                <SectionHeader classes={classes}>📀 Release Info</SectionHeader>
                <InfoRow
                  label="Catalog Number"
                  value={record.catalogNum}
                  classes={classes}
                />
              </>
            )}

            {record.date && (
              <>
                <SectionHeader classes={classes}>📅 Dates</SectionHeader>
                <InfoRow label="Date" value={record.date} classes={classes} />
                <InfoRow label="Original Date" value={record.originalDate} classes={classes} />
                <InfoRow label="Release Date" value={record.releaseDate} classes={classes} />
                <InfoRow label="Original Year" value={record.originalYear} classes={classes} />
                <InfoRow label="Release Year" value={record.releaseYear} classes={classes} />
              </>
            )}

            {tags.length > 0 && (
              <>
                <SectionHeader classes={classes}>🏷️ Additional Tags</SectionHeader>
                {tags.map(([name, values]) => (
                  <InfoRow
                    key={name}
                    label={humanize(underscore(name))}
                    value={values.join(' • ')}
                    classes={classes}
                  />
                ))}
              </>
            )}
          </TableBody>
        </Table>
      </div>

      {/* RAW TAGS TAB */}
      {record.rawTags && (
        <div hidden={tab !== 3} id="raw-tags-body">
          <Table size="small">
            <TableBody>
              <SectionHeader classes={classes}>📄 Raw File Tags (as read from file)</SectionHeader>
              {Object.entries(record.rawTags).sort().map(([key, value]) => (
                <InfoRow
                  key={key}
                  label={key}
                  value={Array.isArray(value) ? value.join(' • ') : value}
                  classes={classes}
                  copyable
                />
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </TableContainer>
  )
}
