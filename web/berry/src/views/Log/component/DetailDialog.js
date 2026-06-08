import { useState } from 'react';
import PropTypes from 'prop-types';

import {
  Button,
  Dialog,
  DialogTitle,
  DialogContent,
  Typography,
  Box,
  IconButton
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import ContentCopy from '@mui/icons-material/ContentCopy';
import UnfoldMore from '@mui/icons-material/UnfoldMore';
import UnfoldLess from '@mui/icons-material/UnfoldLess';
import { copy } from 'utils/common';

function formatJSON(value) {
  if (!value) return null;
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

function DetailField({ label, value, code }) {
  const [expanded, setExpanded] = useState(true);

  return (
    <Box sx={{ mb: 2 }}>
      <Box
        sx={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          mb: 0.5
        }}
      >
        <Typography variant="subtitle2" sx={{ color: 'text.secondary' }}>
          {label}
        </Typography>
        {code && (
          <Box sx={{ display: 'flex', gap: 0.5 }}>
            <Button
              size="small"
              startIcon={<ContentCopy />}
              onClick={() => copy(value, label)}
            >
              复制
            </Button>
            <Button
              size="small"
              startIcon={expanded ? <UnfoldLess /> : <UnfoldMore />}
              onClick={() => setExpanded(!expanded)}
              aria-expanded={expanded}
            >
              {expanded ? '收起' : '展开'}
            </Button>
          </Box>
        )}
      </Box>
      <Typography
        variant="body2"
        component="pre"
        sx={{
          fontFamily: 'monospace',
          fontSize: '0.8rem',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
          bgcolor: 'grey.100',
          p: 1.5,
          borderRadius: 1,
          m: 0
        }}
        style={
          code && !expanded
            ? { maxHeight: 300, overflow: 'auto' }
            : undefined
        }
      >
        {value}
      </Typography>
    </Box>
  );
}

DetailField.propTypes = {
  label: PropTypes.string.isRequired,
  value: PropTypes.string.isRequired,
  code: PropTypes.bool
};

export default function DetailDialog({ open, onClose, logItem }) {
  const fields = [];

  if (!logItem) return null;

  if (logItem.channel_name) {
    fields.push({ label: '渠道名称', value: logItem.channel_name });
  }

  const formattedHeader = formatJSON(logItem.request_header);
  if (formattedHeader) {
    fields.push({ label: '请求头', value: formattedHeader });
  }

  const formattedBody = formatJSON(logItem.request_body);
  if (formattedBody) {
    fields.push({ label: '请求体', value: formattedBody, code: true });
  }

  const formattedResponse = formatJSON(logItem.response_body);
  if (formattedResponse) {
    fields.push({ label: '响应体', value: formattedResponse, code: true });
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>
        详情
        <IconButton
          aria-label="close"
          onClick={onClose}
          sx={{
            position: 'absolute',
            right: 8,
            top: 8,
            color: (theme) => theme.palette.grey[500]
          }}
        >
          <CloseIcon />
        </IconButton>
      </DialogTitle>
      <DialogContent dividers>
        {fields.map((field, index) => (
          <DetailField key={index} label={field.label} value={field.value} code={field.code} />
        ))}
      </DialogContent>
    </Dialog>
  );
}

DetailDialog.propTypes = {
  open: PropTypes.bool.isRequired,
  onClose: PropTypes.func.isRequired,
  logItem: PropTypes.object
};
