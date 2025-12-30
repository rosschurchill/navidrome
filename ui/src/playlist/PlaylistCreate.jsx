import React, { useState, useCallback } from 'react'
import {
  Create,
  SimpleForm,
  TextInput,
  BooleanInput,
  required,
  useTranslate,
  useRefresh,
  useNotify,
  useRedirect,
} from 'react-admin'
import { Title } from '../common'
import SmartPlaylistRulesBuilder from './SmartPlaylistRulesBuilder'

// Custom input wrapper for SmartPlaylistRulesBuilder
const SmartPlaylistInput = ({ record, source }) => {
  const [rules, setRules] = useState(record?.[source] || null)

  const handleChange = useCallback((newRules) => {
    setRules(newRules)
    if (record) {
      record[source] = newRules
    }
  }, [record, source])

  return (
    <SmartPlaylistRulesBuilder
      value={rules}
      onChange={handleChange}
      disabled={false}
    />
  )
}

const PlaylistCreate = (props) => {
  const { basePath } = props
  const refresh = useRefresh()
  const notify = useNotify()
  const redirect = useRedirect()
  const translate = useTranslate()
  const resourceName = translate('resources.playlist.name', { smart_count: 1 })
  const title = translate('ra.page.create', {
    name: `${resourceName}`,
  })

  const onSuccess = () => {
    notify('ra.notification.created', 'info', { smart_count: 1 })
    redirect('list', basePath)
    refresh()
  }

  return (
    <Create title={<Title subTitle={title} />} {...props} onSuccess={onSuccess}>
      <SimpleForm redirect="list" variant={'outlined'}>
        <TextInput source="name" validate={required()} />
        <TextInput multiline source="comment" />
        <BooleanInput source="public" initialValue={true} />
        <SmartPlaylistInput source="rules" />
      </SimpleForm>
    </Create>
  )
}

export default PlaylistCreate
