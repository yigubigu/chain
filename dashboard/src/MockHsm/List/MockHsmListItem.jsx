import React from 'react'

class MockHsmListItem extends React.Component {
  render() {
    const item = this.props.item

    return(
      <tr>
        <td>{item.alias}</td>
        <td><code>{item.xpub}</code></td>
        <td></td>
      </tr>
    )
  }
}

export default MockHsmListItem
