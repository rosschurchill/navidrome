import React, { memo } from 'react'
import Typography from '@material-ui/core/Typography'
import sanitizeFieldRestProps from './sanitizeFieldRestProps'
import md5 from 'blueimp-md5'
import { useRecordContext } from 'react-admin'

// Escape HTML entities to prevent XSS attacks
const escapeHtml = (text) => {
  const htmlEscapes = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  }
  return text.replace(/[&<>"']/g, (char) => htmlEscapes[char])
}

export const MultiLineTextField = memo(
  ({
    className,
    emptyText,
    source,
    firstLine,
    maxLines,
    addLabel,
    ...rest
  }) => {
    const record = useRecordContext(rest)
    const value = record && record[source]
    let lines = value ? value.split('\n') : []
    if (maxLines || firstLine) {
      lines = lines.slice(firstLine, maxLines)
    }

    return (
      <Typography
        className={className}
        variant="body2"
        component="span"
        {...sanitizeFieldRestProps(rest)}
      >
        {lines.length === 0 && emptyText
          ? emptyText
          : lines.map((line, idx) =>
              line === '' ? (
                <br key={md5(line + idx)} />
              ) : (
                <div data-testid={`${source}.${idx}`} key={md5(line + idx)}>
                  {escapeHtml(line)}
                </div>
              ),
            )}
      </Typography>
    )
  },
)

MultiLineTextField.displayName = 'MultiLineTextField'

MultiLineTextField.defaultProps = {
  addLabel: true,
  firstLine: 0,
}
