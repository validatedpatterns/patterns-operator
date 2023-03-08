import * as React from 'react';
import { Button, Modal } from '@patternfly/react-core';
import '../main.css';

export default function PatternsCatalogModel(props) {
  // TODO: Find a better way to check/validate that data comes in
  if (props.data.metadata === undefined) {
    return null;
  }

  return (
    <>
      <Modal
        actions={[
          <Button key="confirm" variant="primary">
            Deploy Pattern
          </Button>,
        ]}
        isOpen={props.isOpen}
        onClose={props.onClose}
        title={props.data.metadata.name}
      >
        <p>{props.data.metadata.name}</p>
        <p>{props.data.spec.description}</p>
      </Modal>
    </>
  );
}
