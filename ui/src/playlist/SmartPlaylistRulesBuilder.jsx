import React, { useState, useCallback, useMemo } from 'react'
import {
  Box,
  Button,
  FormControl,
  FormControlLabel,
  IconButton,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Switch,
  TextField,
  Typography,
  Tooltip,
} from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import AddIcon from '@material-ui/icons/Add'
import DeleteIcon from '@material-ui/icons/Delete'
import { useTranslate } from 'react-admin'
import PropTypes from 'prop-types'

const useStyles = makeStyles((theme) => ({
  root: {
    marginTop: theme.spacing(2),
    marginBottom: theme.spacing(2),
  },
  rulesContainer: {
    padding: theme.spacing(2),
    backgroundColor: theme.palette.background.default,
  },
  ruleRow: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    marginBottom: theme.spacing(1),
    flexWrap: 'wrap',
  },
  fieldSelect: {
    minWidth: 150,
  },
  operatorSelect: {
    minWidth: 120,
  },
  valueInput: {
    minWidth: 150,
    flex: 1,
  },
  groupContainer: {
    padding: theme.spacing(1),
    marginBottom: theme.spacing(1),
    backgroundColor: theme.palette.action.hover,
    borderRadius: theme.shape.borderRadius,
  },
  groupHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: theme.spacing(1),
  },
  conjunctionSelect: {
    minWidth: 80,
  },
  sortSection: {
    marginTop: theme.spacing(2),
    paddingTop: theme.spacing(2),
    borderTop: `1px solid ${theme.palette.divider}`,
    display: 'flex',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
    alignItems: 'center',
  },
  limitInput: {
    width: 100,
  },
}))

// Available fields for smart playlists (matching backend criteria/fields.go)
const FIELDS = {
  title: { type: 'string', label: 'Title' },
  album: { type: 'string', label: 'Album' },
  artist: { type: 'string', label: 'Artist' },
  albumartist: { type: 'string', label: 'Album Artist' },
  genre: { type: 'string', label: 'Genre' },
  year: { type: 'number', label: 'Year' },
  tracknumber: { type: 'number', label: 'Track Number' },
  discnumber: { type: 'number', label: 'Disc Number' },
  duration: { type: 'number', label: 'Duration (s)' },
  bitrate: { type: 'number', label: 'Bitrate' },
  bpm: { type: 'number', label: 'BPM' },
  rating: { type: 'number', label: 'Rating' },
  playcount: { type: 'number', label: 'Play Count' },
  loved: { type: 'boolean', label: 'Loved' },
  compilation: { type: 'boolean', label: 'Compilation' },
  dateadded: { type: 'date', label: 'Date Added' },
  lastplayed: { type: 'date', label: 'Last Played' },
  releasedate: { type: 'date', label: 'Release Date' },
  originaldate: { type: 'date', label: 'Original Date' },
  filetype: { type: 'string', label: 'File Type' },
  filepath: { type: 'string', label: 'File Path' },
  comment: { type: 'string', label: 'Comment' },
  lyrics: { type: 'string', label: 'Lyrics' },
}

// Operators by field type
const OPERATORS = {
  string: [
    { value: 'is', label: 'is' },
    { value: 'isNot', label: 'is not' },
    { value: 'contains', label: 'contains' },
    { value: 'notContains', label: 'does not contain' },
    { value: 'startsWith', label: 'starts with' },
    { value: 'endsWith', label: 'ends with' },
  ],
  number: [
    { value: 'is', label: 'is' },
    { value: 'isNot', label: 'is not' },
    { value: 'gt', label: 'greater than' },
    { value: 'lt', label: 'less than' },
    { value: 'inTheRange', label: 'in range' },
  ],
  boolean: [
    { value: 'is', label: 'is' },
  ],
  date: [
    { value: 'inTheLast', label: 'in the last (days)' },
    { value: 'notInTheLast', label: 'not in the last (days)' },
    { value: 'before', label: 'before' },
    { value: 'after', label: 'after' },
  ],
}

// Sort fields
const SORT_FIELDS = [
  { value: 'title', label: 'Title' },
  { value: 'album', label: 'Album' },
  { value: 'artist', label: 'Artist' },
  { value: 'year', label: 'Year' },
  { value: 'dateadded', label: 'Date Added' },
  { value: 'lastplayed', label: 'Last Played' },
  { value: 'rating', label: 'Rating' },
  { value: 'playcount', label: 'Play Count' },
  { value: 'random', label: 'Random' },
]

// Generate unique ID
const generateId = () => Math.random().toString(36).substr(2, 9)

// Create empty rule
const createEmptyRule = () => ({
  id: generateId(),
  field: 'title',
  operator: 'contains',
  value: '',
})

// Parse existing rules from criteria JSON
const parseRules = (rules) => {
  if (!rules || !rules.Expression) return null

  const parseExpression = (expr, conjunction = 'all') => {
    if (Array.isArray(expr)) {
      return {
        id: generateId(),
        conjunction,
        rules: expr.map((e) => parseExpression(e, 'all')).filter(Boolean),
      }
    }

    // Handle individual rule objects
    for (const [op, conditions] of Object.entries(expr)) {
      if (op === 'all' || op === 'any') {
        return {
          id: generateId(),
          conjunction: op,
          rules: Array.isArray(conditions)
            ? conditions.map((c) => parseExpression(c, op)).filter(Boolean)
            : [],
        }
      }

      // Single condition
      if (typeof conditions === 'object' && conditions !== null) {
        for (const [field, value] of Object.entries(conditions)) {
          return {
            id: generateId(),
            field,
            operator: op,
            value: value,
          }
        }
      }
    }

    return null
  }

  return parseExpression(rules.Expression)
}

// Convert rules back to criteria JSON format
const rulesToCriteria = (group, sort, order, limit) => {
  const buildExpression = (item) => {
    if (item.rules) {
      // It's a group
      const expressions = item.rules.map(buildExpression).filter(Boolean)
      if (expressions.length === 0) return null
      return { [item.conjunction]: expressions }
    }

    // It's a rule
    if (!item.field || !item.operator || item.value === '') return null

    // Handle range operator
    if (item.operator === 'inTheRange') {
      const [min, max] = String(item.value).split('-').map((v) => v.trim())
      return { [item.operator]: { [item.field]: [min, max] } }
    }

    return { [item.operator]: { [item.field]: item.value } }
  }

  const expression = buildExpression(group)
  if (!expression) return null

  return {
    ...expression,
    sort: sort || 'title',
    order: order || 'asc',
    ...(limit > 0 && { limit }),
  }
}

const RuleRow = ({ rule, onUpdate, onDelete, disabled }) => {
  const classes = useStyles()
  const translate = useTranslate()

  const fieldType = FIELDS[rule.field]?.type || 'string'
  const operators = OPERATORS[fieldType] || OPERATORS.string

  const handleFieldChange = useCallback(
    (e) => {
      const newField = e.target.value
      const newFieldType = FIELDS[newField]?.type || 'string'
      const newOperators = OPERATORS[newFieldType]
      const currentOperatorValid = newOperators.some(
        (op) => op.value === rule.operator
      )

      onUpdate({
        ...rule,
        field: newField,
        operator: currentOperatorValid ? rule.operator : newOperators[0].value,
        value: newFieldType === 'boolean' ? true : '',
      })
    },
    [rule, onUpdate]
  )

  const handleOperatorChange = useCallback(
    (e) => onUpdate({ ...rule, operator: e.target.value }),
    [rule, onUpdate]
  )

  const handleValueChange = useCallback(
    (e) => {
      const value =
        fieldType === 'boolean'
          ? e.target.value === 'true'
          : fieldType === 'number'
            ? e.target.value
            : e.target.value
      onUpdate({ ...rule, value })
    },
    [rule, onUpdate, fieldType]
  )

  const renderValueInput = () => {
    if (fieldType === 'boolean') {
      return (
        <FormControl variant="outlined" size="small" className={classes.valueInput}>
          <Select
            value={String(rule.value)}
            onChange={handleValueChange}
            disabled={disabled}
          >
            <MenuItem value="true">{translate('ra.boolean.true')}</MenuItem>
            <MenuItem value="false">{translate('ra.boolean.false')}</MenuItem>
          </Select>
        </FormControl>
      )
    }

    return (
      <TextField
        variant="outlined"
        size="small"
        className={classes.valueInput}
        value={rule.value}
        onChange={handleValueChange}
        disabled={disabled}
        type={fieldType === 'number' || rule.operator === 'inTheLast' || rule.operator === 'notInTheLast' ? 'number' : 'text'}
        placeholder={
          rule.operator === 'inTheRange' ? 'min-max' : ''
        }
      />
    )
  }

  return (
    <Box className={classes.ruleRow}>
      <FormControl variant="outlined" size="small" className={classes.fieldSelect}>
        <Select
          value={rule.field}
          onChange={handleFieldChange}
          disabled={disabled}
        >
          {Object.entries(FIELDS).map(([key, { label }]) => (
            <MenuItem key={key} value={key}>
              {label}
            </MenuItem>
          ))}
        </Select>
      </FormControl>

      <FormControl variant="outlined" size="small" className={classes.operatorSelect}>
        <Select
          value={rule.operator}
          onChange={handleOperatorChange}
          disabled={disabled}
        >
          {operators.map(({ value, label }) => (
            <MenuItem key={value} value={value}>
              {label}
            </MenuItem>
          ))}
        </Select>
      </FormControl>

      {renderValueInput()}

      <Tooltip title={translate('ra.action.delete')}>
        <IconButton size="small" onClick={onDelete} disabled={disabled}>
          <DeleteIcon fontSize="small" />
        </IconButton>
      </Tooltip>
    </Box>
  )
}

RuleRow.propTypes = {
  rule: PropTypes.object.isRequired,
  onUpdate: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
  disabled: PropTypes.bool,
}

const RuleGroup = ({ group, onUpdate, onDelete, isRoot, disabled }) => {
  const classes = useStyles()
  const translate = useTranslate()

  const handleConjunctionChange = useCallback(
    (e) => onUpdate({ ...group, conjunction: e.target.value }),
    [group, onUpdate]
  )

  const handleAddRule = useCallback(() => {
    onUpdate({
      ...group,
      rules: [...group.rules, createEmptyRule()],
    })
  }, [group, onUpdate])

  const handleAddGroup = useCallback(() => {
    onUpdate({
      ...group,
      rules: [
        ...group.rules,
        {
          id: generateId(),
          conjunction: 'all',
          rules: [createEmptyRule()],
        },
      ],
    })
  }, [group, onUpdate])

  const handleRuleUpdate = useCallback(
    (index) => (updatedRule) => {
      const newRules = [...group.rules]
      newRules[index] = updatedRule
      onUpdate({ ...group, rules: newRules })
    },
    [group, onUpdate]
  )

  const handleRuleDelete = useCallback(
    (index) => () => {
      const newRules = group.rules.filter((_, i) => i !== index)
      onUpdate({ ...group, rules: newRules })
    },
    [group, onUpdate]
  )

  return (
    <Paper variant="outlined" className={classes.groupContainer}>
      <Box className={classes.groupHeader}>
        <Box display="flex" alignItems="center" gap={1}>
          <Typography variant="body2">
            {translate('resources.playlist.fields.matchRules')}
          </Typography>
          <FormControl variant="outlined" size="small" className={classes.conjunctionSelect}>
            <Select
              value={group.conjunction}
              onChange={handleConjunctionChange}
              disabled={disabled}
            >
              <MenuItem value="all">{translate('resources.playlist.fields.all')}</MenuItem>
              <MenuItem value="any">{translate('resources.playlist.fields.any')}</MenuItem>
            </Select>
          </FormControl>
          <Typography variant="body2">
            {translate('resources.playlist.fields.ofTheFollowing')}
          </Typography>
        </Box>

        {!isRoot && (
          <Tooltip title={translate('ra.action.delete')}>
            <IconButton size="small" onClick={onDelete} disabled={disabled}>
              <DeleteIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        )}
      </Box>

      {group.rules.map((item, index) =>
        item.rules ? (
          <RuleGroup
            key={item.id}
            group={item}
            onUpdate={handleRuleUpdate(index)}
            onDelete={handleRuleDelete(index)}
            isRoot={false}
            disabled={disabled}
          />
        ) : (
          <RuleRow
            key={item.id}
            rule={item}
            onUpdate={handleRuleUpdate(index)}
            onDelete={handleRuleDelete(index)}
            disabled={disabled}
          />
        )
      )}

      <Box display="flex" gap={1} mt={1}>
        <Button
          size="small"
          startIcon={<AddIcon />}
          onClick={handleAddRule}
          disabled={disabled}
        >
          {translate('resources.playlist.actions.addRule')}
        </Button>
        <Button
          size="small"
          startIcon={<AddIcon />}
          onClick={handleAddGroup}
          disabled={disabled}
        >
          {translate('resources.playlist.actions.addGroup')}
        </Button>
      </Box>
    </Paper>
  )
}

RuleGroup.propTypes = {
  group: PropTypes.object.isRequired,
  onUpdate: PropTypes.func.isRequired,
  onDelete: PropTypes.func,
  isRoot: PropTypes.bool,
  disabled: PropTypes.bool,
}

const SmartPlaylistRulesBuilder = ({ value, onChange, disabled }) => {
  const classes = useStyles()
  const translate = useTranslate()

  const [isSmartPlaylist, setIsSmartPlaylist] = useState(() => {
    return value && value.Expression !== undefined
  })

  const [rulesGroup, setRulesGroup] = useState(() => {
    if (value && value.Expression) {
      return parseRules(value) || {
        id: generateId(),
        conjunction: 'all',
        rules: [createEmptyRule()],
      }
    }
    return {
      id: generateId(),
      conjunction: 'all',
      rules: [createEmptyRule()],
    }
  })

  const [sort, setSort] = useState(() => value?.Sort || 'title')
  const [order, setOrder] = useState(() => value?.Order || 'asc')
  const [limit, setLimit] = useState(() => value?.Limit || 0)

  const handleToggleSmartPlaylist = useCallback(
    (e) => {
      const enabled = e.target.checked
      setIsSmartPlaylist(enabled)
      if (!enabled) {
        onChange(null)
      } else {
        const criteria = rulesToCriteria(rulesGroup, sort, order, limit)
        onChange(criteria)
      }
    },
    [onChange, rulesGroup, sort, order, limit]
  )

  const handleRulesUpdate = useCallback(
    (newGroup) => {
      setRulesGroup(newGroup)
      if (isSmartPlaylist) {
        const criteria = rulesToCriteria(newGroup, sort, order, limit)
        onChange(criteria)
      }
    },
    [isSmartPlaylist, sort, order, limit, onChange]
  )

  const handleSortChange = useCallback(
    (e) => {
      const newSort = e.target.value
      setSort(newSort)
      if (isSmartPlaylist) {
        const criteria = rulesToCriteria(rulesGroup, newSort, order, limit)
        onChange(criteria)
      }
    },
    [isSmartPlaylist, rulesGroup, order, limit, onChange]
  )

  const handleOrderChange = useCallback(
    (e) => {
      const newOrder = e.target.value
      setOrder(newOrder)
      if (isSmartPlaylist) {
        const criteria = rulesToCriteria(rulesGroup, sort, newOrder, limit)
        onChange(criteria)
      }
    },
    [isSmartPlaylist, rulesGroup, sort, limit, onChange]
  )

  const handleLimitChange = useCallback(
    (e) => {
      const newLimit = parseInt(e.target.value, 10) || 0
      setLimit(newLimit)
      if (isSmartPlaylist) {
        const criteria = rulesToCriteria(rulesGroup, sort, order, newLimit)
        onChange(criteria)
      }
    },
    [isSmartPlaylist, rulesGroup, sort, order, onChange]
  )

  return (
    <Box className={classes.root}>
      <FormControlLabel
        control={
          <Switch
            checked={isSmartPlaylist}
            onChange={handleToggleSmartPlaylist}
            disabled={disabled}
            color="primary"
          />
        }
        label={translate('resources.playlist.fields.smartPlaylist')}
      />

      {isSmartPlaylist && (
        <Paper variant="outlined" className={classes.rulesContainer}>
          <RuleGroup
            group={rulesGroup}
            onUpdate={handleRulesUpdate}
            isRoot={true}
            disabled={disabled}
          />

          <Box className={classes.sortSection}>
            <FormControl variant="outlined" size="small">
              <InputLabel>{translate('resources.playlist.fields.sortBy')}</InputLabel>
              <Select
                value={sort}
                onChange={handleSortChange}
                disabled={disabled}
                label={translate('resources.playlist.fields.sortBy')}
                style={{ minWidth: 120 }}
              >
                {SORT_FIELDS.map(({ value, label }) => (
                  <MenuItem key={value} value={value}>
                    {label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            <FormControl variant="outlined" size="small">
              <InputLabel>{translate('resources.playlist.fields.order')}</InputLabel>
              <Select
                value={order}
                onChange={handleOrderChange}
                disabled={disabled}
                label={translate('resources.playlist.fields.order')}
                style={{ minWidth: 100 }}
              >
                <MenuItem value="asc">{translate('resources.playlist.fields.ascending')}</MenuItem>
                <MenuItem value="desc">{translate('resources.playlist.fields.descending')}</MenuItem>
              </Select>
            </FormControl>

            <TextField
              variant="outlined"
              size="small"
              type="number"
              label={translate('resources.playlist.fields.limit')}
              value={limit || ''}
              onChange={handleLimitChange}
              disabled={disabled}
              className={classes.limitInput}
              inputProps={{ min: 0 }}
              placeholder="0 = no limit"
            />
          </Box>
        </Paper>
      )}
    </Box>
  )
}

SmartPlaylistRulesBuilder.propTypes = {
  value: PropTypes.object,
  onChange: PropTypes.func.isRequired,
  disabled: PropTypes.bool,
}

export default SmartPlaylistRulesBuilder
