import * as React from 'react';
import { Button, Modal } from '@patternfly/react-core';
import '../main.css';

export default function PatternsCatalogModel(props) {
  // TODO: Find a better way to check/validate that data comes in
  if (props.data.metadata === undefined) {
    return null;
  }

  const data = props.data.spec

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
        <h1>Organization</h1>
        <p>Name: {data.organization.name}</p>
        <p>Description: {data.organization.description}</p>
        <p>URL: {data.organization.url}</p>
        <p>Maintainers: {data.organization.maintainers}</p>

        <h1>Pattern</h1>
        <p>Name: {data.pattern.name}</p>
        <p>Description: {data.pattern.longDescription}</p>
        <p>Branch: {data.pattern.branch}</p>
        <p>Type: {data.pattern.badge}</p>
        <p>URL: {data.pattern.url}</p>
      </Modal>
    </>
  );
}
