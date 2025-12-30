import React, { useState, useCallback } from 'react'
import {
  Edit,
  FormDataConsumer,
  SimpleForm,
  TextInput,
  TextField,
  BooleanInput,
  required,
  useTranslate,
  usePermissions,
  ReferenceInput,
  SelectInput,
} from 'react-admin'
import { isWritable, Title } from '../common'
import SmartPlaylistRulesBuilder from './SmartPlaylistRulesBuilder'

// Custom input wrapper for SmartPlaylistRulesBuilder in edit mode
const SmartPlaylistInput = ({ record, source }) => {
  const [rules, setRules] = useState(record?.[source] || null)
  const isSmartPlaylist = record?.rules !== null && record?.rules !== undefined

  const handleChange = useCallback((newRules) => {
    setRules(newRules)
    if (record) {
      record[source] = newRules
    }
  }, [record, source])

  // Don't allow switching from regular to smart for existing playlists with tracks
  const disabled = !isSmartPlaylist && record?.songCount > 0

  return (
    <SmartPlaylistRulesBuilder
      value={rules}
      onChange={handleChange}
      disabled={disabled}
    />
  )
}

const SyncFragment = ({ formData, variant, ...rest }) => {
  return (
    <>
      {formData.path && <BooleanInput source="sync" {...rest} />}
      {formData.path && <TextField source="path" {...rest} />}
    </>
  )
}

const PlaylistTitle = ({ record }) => {
  const translate = useTranslate()
  const resourceName = translate('resources.playlist.name', { smart_count: 1 })
  return <Title subTitle={`${resourceName} "${record ? record.name : ''}"`} />
}

const PlaylistEditForm = (props) => {
  const { record } = props
  const { permissions } = usePermissions()
  return (
    <SimpleForm redirect="list" variant={'outlined'} {...props}>
      <TextInput source="name" validate={required()} />
      <TextInput multiline source="comment" />
      {permissions === 'admin' ? (
        <ReferenceInput
          source="ownerId"
          reference="user"
          perPage={0}
          sort={{ field: 'name', order: 'ASC' }}
        >
          <SelectInput
            label={'resources.playlist.fields.ownerName'}
            optionText="userName"
          />
        </ReferenceInput>
      ) : (
        <TextField source="ownerName" />
      )}
      <BooleanInput source="public" disabled={!isWritable(record.ownerId)} />
      <SmartPlaylistInput source="rules" record={record} />
      <FormDataConsumer>
        {(formDataProps) => <SyncFragment {...formDataProps} />}
      </FormDataConsumer>
    </SimpleForm>
  )
}

const PlaylistEdit = (props) => (
  <Edit title={<PlaylistTitle />} actions={false} {...props}>
    <PlaylistEditForm {...props} />
  </Edit>
)

export default PlaylistEdit
