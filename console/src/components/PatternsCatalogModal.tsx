import * as React from 'react';
import { Modal } from '@patternfly/react-core';
import '../main.css';

export default function PatternsCatalogModel(props) {
  /*
  if (props.visible === false) {
    return null;
  }
  */

  if (props.data.metadata === undefined) {
    return null
  }

  return (
    <>
      <Modal isOpen={props.isOpen} onClose={props.onClose} title={props.data.metadata.name}>
        <p>{props.data.metadata.name}</p>
        <p>{props.data.spec.description}</p>
      </Modal>
    </>
  );
}
