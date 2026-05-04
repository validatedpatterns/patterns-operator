import * as React from 'react';
import { useTranslation } from 'react-i18next';
import {
  FileUpload,
  FileUploadHelperText,
  HelperText,
  HelperTextItem,
} from '@patternfly/react-core';

interface FileFieldProps {
  field: {
    name: string;
    description?: string;
    path?: string;
    base64?: boolean;
  };
  value: string | null;
  onChange: (value: string | null) => void;
  /** Shown under the control; also sets FileUpload `validated` to error when set. */
  fieldError?: string;
}

export const FileField: React.FC<FileFieldProps> = ({ field, value, onChange, fieldError }) => {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');
  const [filename, setFilename] = React.useState('');

  const handleFileInputChange = (_, file: File) => {
    setFilename(file.name);

    const reader = new FileReader();
    reader.readAsDataURL(file);
    reader.onload = () => {
      const base64filecontent = reader.result.toString().split(',')[1]; // Remove data:type;base64, prefix
      onChange(base64filecontent);
    };
  };

  const handleClear = () => {
    setFilename('');
    onChange(null);
  };

  return (
    <>
      <FileUpload
        id={`file-${field.name}`}
        value={value}
        filename={filename}
        type="text"
        isRequired={true}
        hideDefaultPreview
        validated={fieldError ? 'error' : 'default'}
        filenamePlaceholder={t('Drag and drop a file or upload one')}
        onFileInputChange={handleFileInputChange}
        onClearClick={handleClear}
        dropzoneProps={{
          accept: {
            'text/*': ['.txt', '.pem', '.crt', '.key', '.ini', '.conf', '.config'],
            'application/*': ['.json', '.yaml', '.yml'],
          },
        }}
      >
        <FileUploadHelperText>
          <HelperText>
            {fieldError && (
              <HelperTextItem variant="error" id={`file-${field.name}-error`}>
                {fieldError}
              </HelperTextItem>
            )}
            {field.description && (
              <HelperTextItem id={`file-${field.name}-helpertext`}>{field.description}</HelperTextItem>
            )}
          </HelperText>
        </FileUploadHelperText>
      </FileUpload>
    </>
  );
};
