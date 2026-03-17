import * as React from 'react';
import { useTranslation } from 'react-i18next';
import { FileUpload, FormHelperText, HelperText, HelperTextItem } from '@patternfly/react-core';

interface FileFieldProps {
  field: {
    name: string;
    description?: string;
    path?: string;
    base64?: boolean;
  };
  value: File | string | null;
  onChange: (value: File | string | null) => void;
}

export const FileField: React.FC<FileFieldProps> = ({ field, value, onChange }) => {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');
  const [filename, setFilename] = React.useState('');
  const [isLoading, setIsLoading] = React.useState(false);

  const handleFileInputChange = (file: File | null, filename: string) => {
    setFilename(filename);
    if (!file) {
      onChange(null);
      return;
    }

    setIsLoading(true);

    if (field.base64) {
      // Read file as base64
      const reader = new FileReader();
      reader.onload = () => {
        const result = reader.result as string;
        const base64 = result.split(',')[1]; // Remove data:type;base64, prefix
        onChange(base64);
        setIsLoading(false);
      };
      reader.onerror = () => {
        onChange(null);
        setIsLoading(false);
      };
      reader.readAsDataURL(file);
    } else {
      // Read file as text
      const reader = new FileReader();
      reader.onload = () => {
        onChange(reader.result as string);
        setIsLoading(false);
      };
      reader.onerror = () => {
        onChange(null);
        setIsLoading(false);
      };
      reader.readAsText(file);
    }
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
        onChange={handleFileInputChange}
        onClearClick={handleClear}
        isLoading={isLoading}
        dropzoneProps={{
          accept: {
            'text/*': ['.txt', '.pem', '.crt', '.key', '.ini', '.conf', '.config'],
            'application/*': ['.json', '.yaml', '.yml'],
          },
        }}
      />
      <FormHelperText>
        <HelperText>
          <HelperTextItem>
            {field.description ||
              (field.base64
                ? t('Upload a file that will be base64 encoded')
                : t('Upload a text file'))}
          </HelperTextItem>
          {field.path && (
            <HelperTextItem>
              {t('File will be stored at: {{path}}', { path: field.path })}
            </HelperTextItem>
          )}
          {field.base64 && (
            <HelperTextItem>
              {t('File content will be automatically base64 encoded')}
            </HelperTextItem>
          )}
        </HelperText>
      </FormHelperText>
    </>
  );
};
