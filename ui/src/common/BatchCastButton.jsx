import React from 'react'
import { Button, useTranslate, useUnselectAll } from 'react-admin'
import { useDispatch } from 'react-redux'
import { openSonosCastDialog } from '../actions'
import SpeakerIcon from '@material-ui/icons/Speaker'

export const BatchCastButton = ({ resource, selectedIds, className }) => {
  const dispatch = useDispatch()
  const translate = useTranslate()
  const unselectAll = useUnselectAll()

  const cast = () => {
    dispatch(
      openSonosCastDialog({
        selectedIds,
        resource,
        name: translate('ra.action.bulk_actions', {
          _: 'ra.action.bulk_actions',
          smart_count: selectedIds.length,
        }),
      }),
    )
    unselectAll(resource)
  }

  const caption = translate('resources.song.actions.cast')
  return (
    <Button
      aria-label={caption}
      onClick={cast}
      label={caption}
      className={className}
    >
      <SpeakerIcon />
    </Button>
  )
}

BatchCastButton.propTypes = {}
